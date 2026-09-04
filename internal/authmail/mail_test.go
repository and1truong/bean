package authmail

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type failingSender struct{}

func (failingSender) Send(_ context.Context, m Message) error {
	return errors.New("provider leaked " + m.Link)
}
func validConfig() Config {
	return Config{Address: "localhost:587", From: "bean@example.test", Origin: "https://example.test", Key: base64.StdEncoding.EncodeToString(make([]byte, 32))}
}
func TestEncryptedEnvelopeAndDeliveryRedaction(t *testing.T) {
	service, err := New(validConfig(), failingSender{})
	if err != nil {
		t.Fatal(err)
	}
	message := Envelope{ID: "request", AppID: "app", ReleaseID: "release", Email: "member@example.test", Token: strings.Repeat("a", 43), Expires: time.Now().Add(time.Minute)}
	payload, err := service.Seal(DeliveryTopic, message)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload["sealed"].(string), message.Email) {
		t.Fatal("plaintext address")
	}
	opened, err := service.Open(DeliveryTopic, payload)
	if err != nil || opened.Token != message.Token {
		t.Fatal(err)
	}
	if _, err := service.Open(RequestTopic, payload); err == nil {
		t.Fatal("cross-topic envelope accepted")
	}
	payload["sealed"] = "tampered"
	if _, err := service.Open(DeliveryTopic, payload); err == nil {
		t.Fatal("tampered envelope accepted")
	}
	if err := service.Send(context.Background(), message); err != ErrDelivery {
		t.Fatal("provider error not redacted", err)
	}
}
func TestMailConfigurationFailsClosed(t *testing.T) {
	for _, origin := range []string{"http://public.example", "https://user:secret@example.test", "https://example.test/path", "https://example.test?token=secret"} {
		config := validConfig()
		config.Origin = origin
		if _, err := New(config, failingSender{}); err != ErrConfiguration {
			t.Fatal(origin, err)
		}
	}
	for _, raw := range []string{"secret", "{}", `{"password":"do-not-print","unknown":true}`, `{} {}`} {
		if _, err := FromEnvironment(raw); err != ErrConfiguration {
			t.Fatal("configuration not rejected safely", err)
		}
	}
	if service, err := FromEnvironment(""); service != nil || err != nil {
		t.Fatal("email-free startup broken")
	}
	if ValidEmail("member@example.test\r\nBcc: attacker@example.test") {
		t.Fatal("header injection accepted")
	}
}
func TestSMTPRefusesPlaintextServerBeforeSendingAddressOrCredentials(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	transcript := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			transcript <- ""
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		_, _ = conn.Write([]byte("220 localhost ESMTP\r\n"))
		reader := bufio.NewReader(conn)
		line, _ := reader.ReadString('\n')
		_, _ = conn.Write([]byte("250 localhost\r\n"))
		rest, _ := reader.ReadString('\n')
		transcript <- line + rest
	}()
	config := validConfig()
	config.Address = listener.Addr().String()
	config.Username = "secret-user"
	config.Password = "secret-password"
	service, err := New(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = service.Send(context.Background(), Envelope{Email: "member@example.test", Token: strings.Repeat("a", 43), Expires: time.Now().Add(time.Minute)})
	if err != ErrDelivery {
		t.Fatal("plaintext SMTP accepted", err)
	}
	sent := <-transcript
	if strings.Contains(sent, "AUTH") || strings.Contains(sent, "MAIL") || strings.Contains(sent, "secret") {
		t.Fatal("sent credentials before TLS")
	}
}
