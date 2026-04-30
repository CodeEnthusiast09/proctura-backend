package mailer

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "templates/*.html"))

type inviteData struct {
	FirstName  string
	InviteLink string
}

type resetData struct {
	FirstName string
	ResetLink string
}

type loginNotificationData struct {
	FirstName string
	LoginTime string
	IP        string
	Location  string
}

func renderTemplate(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
