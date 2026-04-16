// internal/mailer/mailer.go
package mailer

// Mailer is the interface every email backend must satisfy.
// Swap implementations by injecting a different struct at startup.
type Mailer interface {
	SendInvite(to, firstName, inviteLink string) error
	SendPasswordReset(to, firstName, resetLink string) error
}
