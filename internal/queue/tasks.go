package queue

import "encoding/json"

// Task type identifiers — used by both client (enqueue) and server (process).
const (
	TypeSendInvite            = "email:send_invite"
	TypeSendPasswordReset     = "email:send_password_reset"
	TypeSendLoginNotification = "email:send_login_notification"
	TypeGradeSubmission       = "submission:grade"
)

// SendInvitePayload carries the data needed to render and send an invite email.
type SendInvitePayload struct {
	To         string `json:"to"`
	FirstName  string `json:"first_name"`
	InviteLink string `json:"invite_link"`
}

func (p SendInvitePayload) Marshal() ([]byte, error) { return json.Marshal(p) }

// SendPasswordResetPayload carries the data needed for a password reset email.
type SendPasswordResetPayload struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
	ResetLink string `json:"reset_link"`
}

func (p SendPasswordResetPayload) Marshal() ([]byte, error) { return json.Marshal(p) }

// SendLoginNotificationPayload carries the data for a login alert email.
type SendLoginNotificationPayload struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
	LoginTime string `json:"login_time"`
	IP        string `json:"ip"`
	Location  string `json:"location"`
}

func (p SendLoginNotificationPayload) Marshal() ([]byte, error) { return json.Marshal(p) }

// GradeSubmissionPayload identifies a submission to grade.
type GradeSubmissionPayload struct {
	SubmissionID string `json:"submission_id"`
}

func (p GradeSubmissionPayload) Marshal() ([]byte, error) { return json.Marshal(p) }
