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

	// Submission service runs grading inline here — it does NOT enqueue, since
	// this *is* the consumer. Leaving WithGradingEnqueuer unset means
	// dispatchGrading would fall back to a goroutine, but the worker calls
	// GradeSubmission directly so that path is never hit.
	judge0Client := submission.NewJudge0Client(cfg.Judge0)
	submissionSvc := submission.NewService(db, judge0Client)

	mux := queue.NewServeMux()
	mux.HandleFunc(queue.TypeSendInvite, makeSendInviteHandler(deliveryMailer))
	mux.HandleFunc(queue.TypeSendPasswordReset, makeSendPasswordResetHandler(deliveryMailer))
	mux.HandleFunc(queue.TypeSendLoginNotification, makeSendLoginNotificationHandler(deliveryMailer))
	mux.HandleFunc(queue.TypeGradeSubmission, makeGradeSubmissionHandler(submissionSvc))

	srv := queue.NewServer(cfg.Redis)
	log.Printf("[worker] processing tasks from redis %s", cfg.Redis.Addr)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("worker run: %v", err)
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
