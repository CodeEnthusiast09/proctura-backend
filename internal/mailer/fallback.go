package mailer

import "log"

// FallbackMailer tries each provider in order, stopping at the first success.
type FallbackMailer struct {
	providers []Mailer
}

func NewFallbackMailer(providers ...Mailer) *FallbackMailer {
	return &FallbackMailer{providers: providers}
}

func (f *FallbackMailer) SendInvite(to, firstName, inviteLink string) error {
	return f.try(func(m Mailer) error { return m.SendInvite(to, firstName, inviteLink) })
}

func (f *FallbackMailer) SendPasswordReset(to, firstName, resetLink string) error {
	return f.try(func(m Mailer) error { return m.SendPasswordReset(to, firstName, resetLink) })
}

func (f *FallbackMailer) SendLoginNotification(to, firstName, loginTime, ip, location string) error {
	return f.try(func(m Mailer) error {
		return m.SendLoginNotification(to, firstName, loginTime, ip, location)
	})
}

func (f *FallbackMailer) try(send func(Mailer) error) error {
	var lastErr error
	for i, p := range f.providers {
		if err := send(p); err != nil {
			log.Printf("[mailer] provider %d failed: %v — trying next", i, err)
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}
