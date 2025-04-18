package email

import (
	"bytes"
	"embed"
	"html/template"
	"strconv"

	"gopkg.in/gomail.v2"
)

//go:embed template/otp.html
var emailTemplates embed.FS

type EmailService struct {
	host     string
	port     string
	username string
	password string
}

func NewEmailService(host, port, username, password string) *EmailService {
	return &EmailService{
		host:     host,
		port:     port,
		username: username,
		password: password,
	}
}

type EmailData struct {
	PIN  string
	ICON string
}

func (s *EmailService) SendVerificationEmail(to, otp string) error {
	port, _ := strconv.Atoi(s.port)
	d := gomail.NewDialer(s.host, port, s.username, s.password)

	// Read the template file
	templateContent, err := emailTemplates.ReadFile("template/otp.html")
	if err != nil {
		return err
	}

	// Parse the template
	tmpl, err := template.New("otp").Parse(string(templateContent))
	if err != nil {
		return err
	}

	// Prepare email data
	data := EmailData{
		PIN:  otp,
		ICON: "https://ak47album.com/album-cover.jpg", // You can replace this with a base64 encoded image if needed
	}

	// Execute the template
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", s.username)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Your Verification Code")
	m.SetBody("text/html", body.String())

	return d.DialAndSend(m)
}
