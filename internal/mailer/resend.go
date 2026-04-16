// internal/mailer/resend.go
package mailer

import (
	"fmt"

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
	_, err := m.client.Emails.Send(&resend.SendEmailRequest{
		From:    m.from,
		To:      []string{to},
		Subject: "You've been invited to Proctura",
		Html:    inviteHTML(firstName, inviteLink),
	})
	if err != nil {
		return fmt.Errorf("resend send invite: %w", err)
	}
	return nil
}

func (m *ResendMailer) SendPasswordReset(to, firstName, resetLink string) error {
	_, err := m.client.Emails.Send(&resend.SendEmailRequest{
		From:    m.from,
		To:      []string{to},
		Subject: "Reset your Proctura password",
		Html:    resetHTML(firstName, resetLink),
	})
	if err != nil {
		return fmt.Errorf("resend send password reset: %w", err)
	}
	return nil
}

// ── Email templates ───────────────────────────────────────────────────────────

func inviteHTML(firstName, inviteLink string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;background:#f8fafc;padding:40px 0;margin:0">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:40px;border:1px solid #e2e8f0">
    <h1 style="font-size:22px;font-weight:700;color:#0f172a;margin:0 0 8px">
      Welcome to Proctura, %s
    </h1>
    <p style="color:#64748b;font-size:15px;margin:0 0 28px;line-height:1.6">
      You've been invited to join Proctura — an online coding exam platform.
      Click the button below to set up your password and activate your account.
    </p>
    <a href="%s"
       style="display:inline-block;background:#1e3a5f;color:#fff;font-weight:600;
              font-size:14px;padding:12px 24px;border-radius:8px;text-decoration:none">
      Set Up Your Account
    </a>
    <p style="color:#94a3b8;font-size:12px;margin:28px 0 0;line-height:1.6">
      This link expires in 7 days. If you didn't expect this invitation, you can safely ignore this email.
    </p>
  </div>
</body>
</html>`, firstName, inviteLink)
}

func resetHTML(firstName, resetLink string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;background:#f8fafc;padding:40px 0;margin:0">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:40px;border:1px solid #e2e8f0">
    <h1 style="font-size:22px;font-weight:700;color:#0f172a;margin:0 0 8px">
      Reset your password, %s
    </h1>
    <p style="color:#64748b;font-size:15px;margin:0 0 28px;line-height:1.6">
      We received a request to reset your Proctura password.
      Click the button below to choose a new password.
    </p>
    <a href="%s"
       style="display:inline-block;background:#1e3a5f;color:#fff;font-weight:600;
              font-size:14px;padding:12px 24px;border-radius:8px;text-decoration:none">
      Reset Password
    </a>
    <p style="color:#94a3b8;font-size:12px;margin:28px 0 0;line-height:1.6">
      This link expires in 1 hour. If you didn't request a password reset, you can safely ignore this email.
    </p>
  </div>
</body>
</html>`, firstName, resetLink)
}
