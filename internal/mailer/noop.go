// internal/mailer/noop.go
package mailer

import "log"

// NoOpMailer logs emails instead of sending them.
// Used in tests and when no API key is configured.
type NoOpMailer struct{}

func (m *NoOpMailer) SendInvite(to, firstName, inviteLink string) error {
	log.Printf("[mailer:noop] invite → %s | link: %s", to, inviteLink)
	return nil
}

func (m *NoOpMailer) SendPasswordReset(to, firstName, resetLink string) error {
	log.Printf("[mailer:noop] password reset → %s | link: %s", to, resetLink)
	return nil
}
