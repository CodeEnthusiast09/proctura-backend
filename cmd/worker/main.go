package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/joho/godotenv"

	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/CodeEnthusiast09/proctura-backend/internal/database"
	"github.com/CodeEnthusiast09/proctura-backend/internal/mailer"
	"github.com/CodeEnthusiast09/proctura-backend/internal/queue"
	"github.com/CodeEnthusiast09/proctura-backend/internal/submission"
	"github.com/hibiken/asynq"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("[env] no .env file found, using system environment")
	}

	cfg := config.Load()

	db, err := database.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	// The worker uses the actual delivery mailer (Resend → SMTP fallback → no-op).
	// The API process uses QueueMailer instead — every send is just an enqueue.
	deliveryMailer := buildDeliveryMailer(cfg)

	// The worker consumes grading tasks, but it also now produces them: the
	// stale sweep marks expired submissions and hands each one off for grading.
	// Wiring the enqueuer means those go back through the queue, where asynq's
	// concurrency limit bounds them, instead of dispatchGrading falling back to
	// one unbounded goroutine per submission. A lab losing power mid-exam would
	// otherwise fire a Judge0 call for every student at once.
	queueClient := queue.NewClient(cfg.Redis)
	defer queueClient.Close()

	judge0Client := submission.NewJudge0Client(cfg.Judge0)
	submissionSvc := submission.NewService(db, judge0Client).WithGradingEnqueuer(queueClient)

	mux := queue.NewServeMux()
	mux.HandleFunc(queue.TypeSendInvite, makeSendInviteHandler(deliveryMailer))
	mux.HandleFunc(queue.TypeSendPasswordReset, makeSendPasswordResetHandler(deliveryMailer))
	mux.HandleFunc(queue.TypeSendLoginNotification, makeSendLoginNotificationHandler(deliveryMailer))
	mux.HandleFunc(queue.TypeGradeSubmission, makeGradeSubmissionHandler(submissionSvc))
	mux.HandleFunc(queue.TypeSweepStale, makeSweepStaleHandler(submissionSvc))

	scheduler := queue.NewScheduler(cfg.Redis)
	if _, err := scheduler.Register(
		"@every "+cfg.Submission.SweepInterval.String(),
		asynq.NewTask(queue.TypeSweepStale, nil),
		asynq.Queue(queue.QueueDefault),
	); err != nil {
		log.Fatalf("register sweep schedule: %v", err)
	}

	go func() {
		log.Printf("[worker] stale-submission sweep every %s", cfg.Submission.SweepInterval)
		if err := scheduler.Run(); err != nil {
			log.Fatalf("scheduler run: %v", err)
		}
	}()

	srv := queue.NewServer(cfg.Redis)
	log.Printf("[worker] processing tasks from redis %s", cfg.Redis.Addr)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("worker run: %v", err)
	}
}

// makeSweepStaleHandler closes out submissions whose exam time ran out but
// whose browser never submitted them. Carries no payload — the schedule is the
// only trigger.
func makeSweepStaleHandler(svc *submission.Service) asynq.HandlerFunc {
	return func(_ context.Context, _ *asynq.Task) error {
		swept, err := svc.SweepExpired()
		if err != nil {
			return err
		}
		if swept > 0 {
			log.Printf("[sweep] closed out %d expired submission(s)", swept)
		}
		return nil
	}
}

func buildDeliveryMailer(cfg *config.Config) mailer.Mailer {
	var providers []mailer.Mailer
	if cfg.Email.ResendAPIKey != "" {
		providers = append(providers, mailer.NewResendMailer(cfg.Email.ResendAPIKey, cfg.Email.From))
	}
	if cfg.SMTP.Host != "" && cfg.SMTP.User != "" {
		providers = append(providers, mailer.NewSMTPMailer(
			cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.User, cfg.SMTP.Password, cfg.Email.From,
		))
	}
	switch len(providers) {
	case 0:
		log.Println("[mailer] no email provider configured — using no-op mailer")
		return &mailer.NoOpMailer{}
	case 1:
		return providers[0]
	default:
		log.Printf("[mailer] %d email providers configured (fallback chain active)", len(providers))
		return mailer.NewFallbackMailer(providers...)
	}
}

func makeSendInviteHandler(m mailer.Mailer) asynq.HandlerFunc {
	return func(_ context.Context, t *asynq.Task) error {
		var p queue.SendInvitePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		return m.SendInvite(p.To, p.FirstName, p.InviteLink)
	}
}

func makeSendPasswordResetHandler(m mailer.Mailer) asynq.HandlerFunc {
	return func(_ context.Context, t *asynq.Task) error {
		var p queue.SendPasswordResetPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		return m.SendPasswordReset(p.To, p.FirstName, p.ResetLink)
	}
}

func makeSendLoginNotificationHandler(m mailer.Mailer) asynq.HandlerFunc {
	return func(_ context.Context, t *asynq.Task) error {
		var p queue.SendLoginNotificationPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		return m.SendLoginNotification(p.To, p.FirstName, p.LoginTime, p.IP, p.Location)
	}
}

func makeGradeSubmissionHandler(svc *submission.Service) asynq.HandlerFunc {
	return func(_ context.Context, t *asynq.Task) error {
		var p queue.GradeSubmissionPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		return svc.GradeSubmission(p.SubmissionID)
	}
}
