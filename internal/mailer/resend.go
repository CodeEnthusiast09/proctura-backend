package mailer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	resend "github.com/resend/resend-go/v2"
)

// ResendMailer sends transactional emails via the Resend API.
type ResendMailer struct {
	client *resend.Client
	from   string
}

func NewResendMailer(apiKey, from string) *ResendMailer {
	return &ResendMailer{
		client: resend.NewClient(apiKey),
		from:   from,
	}
}

func (m *ResendMailer) SendInvite(to, firstName, inviteLink string) error {
	html, err := renderTemplate("invite.html", inviteData{FirstName: firstName, InviteLink: inviteLink})
	if err != nil {
		return fmt.Errorf("render invite template: %w", err)
	}
	_, err = m.client.Emails.Send(&resend.SendEmailRequest{
		From:    m.from,
		To:      []string{to},
		Subject: "You've been invited to Proctura",
		Html:    html,
	})
	if err != nil {
		return fmt.Errorf("resend send invite: %w", err)
	}
	return nil
}

func (m *ResendMailer) SendPasswordReset(to, firstName, resetLink string) error {
	html, err := renderTemplate("reset_password.html", resetData{FirstName: firstName, ResetLink: resetLink})
	if err != nil {
		return fmt.Errorf("render reset template: %w", err)
	}
	_, err = m.client.Emails.Send(&resend.SendEmailRequest{
		From:    m.from,
		To:      []string{to},
		Subject: "Reset your Proctura password",
		Html:    html,
	})
	if err != nil {
		return fmt.Errorf("resend send password reset: %w", err)
	}
	return nil
}

func (m *ResendMailer) SendLoginNotification(to, firstName, loginTime, ip, location string) error {
	html, err := renderTemplate("login_notification.html", loginNotificationData{
		FirstName: firstName,
		LoginTime: loginTime,
		IP:        ip,
		Location:  location,
	})
	if err != nil {
		return fmt.Errorf("render login notification template: %w", err)
	}
	_, err = m.client.Emails.Send(&resend.SendEmailRequest{
		From:    m.from,
		To:      []string{to},
		Subject: "New sign-in to your Proctura account",
		Html:    html,
	})
	if err != nil {
		return fmt.Errorf("resend send login notification: %w", err)
	}
	return nil
}

// LookupLocation attempts to resolve an IP address to "City, Country".
// Falls back to "Unknown location" on any error.
func LookupLocation(ip string) string {
	if ip == "" || strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "::1") || strings.HasPrefix(ip, "192.168.") {
		return "Local network"
	}

	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/" + ip + "?fields=city,country,status")
	if err != nil {
		return "Unknown location"
	}
	defer resp.Body.Close()

	var result struct {
		Status  string `json:"status"`
		City    string `json:"city"`
		Country string `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.Status != "success" {
		return "Unknown location"
	}
	return result.City + ", " + result.Country
}
