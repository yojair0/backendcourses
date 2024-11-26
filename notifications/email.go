package notification

import (
	"log"
	"os"

	"gopkg.in/gomail.v2"
)

type EmailService struct {
	smtpHost string
	smtpPort int
	username string
	password string
}

func NewEmailService() *EmailService {
	return &EmailService{
		smtpHost: os.Getenv("SMTP_HOST"),
		smtpPort: 587, // Cambia si usas otro puerto
		username: os.Getenv("SMTP_USER"),
		password: os.Getenv("SMTP_PASS"),
	}
}

func (s *EmailService) SendMail(to string, subject string, body string) error {
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", s.username)
	mailer.SetHeader("To", to)
	mailer.SetHeader("Subject", subject)
	mailer.SetBody("text/plain", body)

	dialer := gomail.NewDialer(s.smtpHost, s.smtpPort, s.username, s.password)

	if err := dialer.DialAndSend(mailer); err != nil {
		log.Printf("Error sending email: %v", err)
		return err
	}

	log.Printf("Email sent to %s", to)
	return nil
}
