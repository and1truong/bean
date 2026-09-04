package authmail

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

type smtpSender struct {
	config Config
	host   string
	roots  *x509.CertPool
}

func (s *smtpSender) Send(ctx context.Context, message Message) error {
	if !ValidEmail(message.To) {
		return ErrDelivery
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.config.Address)
	if err != nil {
		return ErrDelivery
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return ErrDelivery
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return ErrDelivery
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return ErrDelivery
	}
	if err := client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12, RootCAs: s.roots}); err != nil {
		return ErrDelivery
	}
	if s.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.host)); err != nil {
			return ErrDelivery
		}
	}
	if err := client.Mail(s.config.From); err != nil {
		return ErrDelivery
	}
	if err := client.Rcpt(message.To); err != nil {
		return ErrDelivery
	}
	writer, err := client.Data()
	if err != nil {
		return ErrDelivery
	}
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Reset your password\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nA password reset was requested for your account.\r\nOpen this link to choose a new password. It expires in 15 minutes.\r\n\r\n%s\r\n\r\nIf you did not request this, ignore this message.\r\n", s.config.From, message.To, message.Link)
	if _, err := writer.Write([]byte(body)); err != nil {
		return ErrDelivery
	}
	if err := writer.Close(); err != nil {
		return ErrDelivery
	}
	return client.Quit()
}
