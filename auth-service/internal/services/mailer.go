package services

import (
	"fmt"
	"log"
	"net/smtp"
)

type EmailService interface {
	SendVerificationCode(email, code string) error
}

type SMTPMailer struct {
	Host     string
	Port     string
	Username string
	Password string
}

func NewSMTPMailer(host, port, username, password string) *SMTPMailer {
	return &SMTPMailer{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
	}
}

func (m *SMTPMailer) SendVerificationCode(email, code string) error {
	auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)
	message := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Подтверждение email\r\n\r\nВаш временный пароль: %s",
		m.Username, email, code,
	)
	addr := fmt.Sprintf("%s:%s", m.Host, m.Port)
	err := smtp.SendMail(addr, auth, m.Username, []string{email}, []byte(message))
	if err != nil {
		log.Println("Ошибка при отправке email:", err)
		return err
	}
	return nil
}
