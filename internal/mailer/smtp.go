package mailer

import (
	"fmt"
	"net/smtp"
)

// SMTPMailer sends transactional emails via any standard SMTP server.
type SMTPMailer struct {
	host     string
	port     string
	user     string
	password string
	from     string
}

func NewSMTPMailer(host, port, user, password, from string) *SMTPMailer {
	return &SMTPMailer{host: host, port: port, user: user, password: password, from: from}
}

func (m *SMTPMailer) send(to, subject, html string) error {
	auth := smtp.PlainAuth("", m.user, m.password, m.host)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s",
		m.from, to, subject, html,
	)
	return smtp.SendMail(m.host+":"+m.port, auth, m.user, []string{to}, []byte(msg))
}

func (m *SMTPMailer) SendInvite(to, firstName, inviteLink string) error {
	html, err := renderTemplate("invite.html", inviteData{FirstName: firstName, InviteLink: inviteLink})
	if err != nil {
		return fmt.Errorf("render invite template: %w", err)
	}
	return m.send(to, "You've been invited to Proctura", html)
}

func (m *SMTPMailer) SendPasswordReset(to, firstName, resetLink string) error {
	html, err := renderTemplate("reset_password.html", resetData{FirstName: firstName, ResetLink: resetLink})
	if err != nil {
		return fmt.Errorf("render reset template: %w", err)
	}
	return m.send(to, "Reset your Proctura password", html)
}

func (m *SMTPMailer) SendLoginNotification(to, firstName, loginTime, ip, location string) error {
	html, err := renderTemplate("login_notification.html", loginNotificationData{
		FirstName: firstName,
		LoginTime: loginTime,
		IP:        ip,
		Location:  location,
	})
	if err != nil {
		return fmt.Errorf("render login notification template: %w", err)
	}
	return m.send(to, "New sign-in to your Proctura account", html)
}
