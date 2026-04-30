package mailer

// Mailer is the interface every email backend must satisfy.
// Swap implementations by injecting a different struct at startup.
type Mailer interface {
	SendInvite(to, firstName, inviteLink string) error
	SendPasswordReset(to, firstName, resetLink string) error
	SendLoginNotification(to, firstName, loginTime, ip, location string) error
}

// LoginInfo holds data collected at login time for the notification email.
type LoginInfo struct {
	IP       string
	Location string // "City, Country" from geolocation
}
