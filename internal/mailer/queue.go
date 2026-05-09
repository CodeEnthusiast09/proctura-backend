package mailer

import (
	"github.com/CodeEnthusiast09/proctura-backend/internal/queue"
)

// QueueMailer is a Mailer implementation that enqueues each send onto an
// async queue. Callers (auth/tenant/user services) get the same Mailer
// interface but never block on SMTP/Resend latency. The actual delivery
// happens in the worker, where the real Mailer (Resend, SMTP, …) is wired in.
type QueueMailer struct {
	q *queue.Client
}

func NewQueueMailer(q *queue.Client) *QueueMailer {
	return &QueueMailer{q: q}
}

func (m *QueueMailer) SendInvite(to, firstName, inviteLink string) error {
	return m.q.EnqueueSendInvite(queue.SendInvitePayload{
		To:         to,
		FirstName:  firstName,
		InviteLink: inviteLink,
	})
}

func (m *QueueMailer) SendPasswordReset(to, firstName, resetLink string) error {
	return m.q.EnqueueSendPasswordReset(queue.SendPasswordResetPayload{
		To:        to,
		FirstName: firstName,
		ResetLink: resetLink,
	})
}

func (m *QueueMailer) SendLoginNotification(to, firstName, loginTime, ip, location string) error {
	return m.q.EnqueueSendLoginNotification(queue.SendLoginNotificationPayload{
		To:        to,
		FirstName: firstName,
		LoginTime: loginTime,
		IP:        ip,
		Location:  location,
	})
}
