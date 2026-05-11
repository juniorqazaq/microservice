package provider

import (
	"context"
	"fmt"
	"net/smtp"
)

type RealProvider struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

func NewRealProvider(host, port, user, password, from string) *RealProvider {
	return &RealProvider{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		From:     from,
	}
}

func (p *RealProvider) Send(ctx context.Context, to, subject, body string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	addr := p.Host + ":" + p.Port
	auth := smtp.PlainAuth("", p.User, p.Password, p.Host)
	message := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body))

	return smtp.SendMail(addr, auth, p.From, []string{to}, message)
}
