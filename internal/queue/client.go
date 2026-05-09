package queue

import (
	"fmt"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/hibiken/asynq"
)

// Client wraps asynq.Client with typed enqueue helpers for our task types.
// All tasks default to 3 retries with exponential backoff via the asynq server config.
type Client struct {
	c *asynq.Client
}

func NewClient(cfg config.RedisConfig) *Client {
	return &Client{
		c: asynq.NewClient(asynq.RedisClientOpt{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}),
	}
}

func (q *Client) Close() error {
	return q.c.Close()
}

// EnqueueSendInvite queues an invite email for delivery.
func (q *Client) EnqueueSendInvite(p SendInvitePayload) error {
	return q.enqueue(TypeSendInvite, p)
}

// EnqueueSendPasswordReset queues a password-reset email.
func (q *Client) EnqueueSendPasswordReset(p SendPasswordResetPayload) error {
	return q.enqueue(TypeSendPasswordReset, p)
}

// EnqueueSendLoginNotification queues a login notification email.
func (q *Client) EnqueueSendLoginNotification(p SendLoginNotificationPayload) error {
	return q.enqueue(TypeSendLoginNotification, p)
}

// EnqueueGradeSubmission queues a submission for grading via Judge0.
func (q *Client) EnqueueGradeSubmission(submissionID string) error {
	return q.enqueue(TypeGradeSubmission, GradeSubmissionPayload{SubmissionID: submissionID})
}

type marshaler interface {
	Marshal() ([]byte, error)
}

func (q *Client) enqueue(taskType string, payload marshaler) error {
	body, err := payload.Marshal()
	if err != nil {
		return fmt.Errorf("marshal task payload: %w", err)
	}
	task := asynq.NewTask(taskType, body)
	if _, err := q.c.Enqueue(task,
		asynq.MaxRetry(3),
		asynq.Timeout(2*time.Minute),
	); err != nil {
		return fmt.Errorf("enqueue %s: %w", taskType, err)
	}
	return nil
}
