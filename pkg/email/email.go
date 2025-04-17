package email

import (
	"fmt"
	"strconv"

	"gopkg.in/gomail.v2"
)

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

func (s *EmailService) SendVerificationEmail(to, token string) error {
	port, _ := strconv.Atoi(s.port)
	d := gomail.NewDialer(s.host, port, s.username, s.password)

	m := gomail.NewMessage()
	m.SetHeader("From", s.username)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Email Verification")
	
	verificationLink := fmt.Sprintf("http://localhost:8080/verify?token=%s", token)
	body := fmt.Sprintf("Please click the following link to verify your email: %s", verificationLink)
	
	m.SetBody("text/plain", body)

	return d.DialAndSend(m)
} 