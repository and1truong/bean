// Package authmail delivers host-configured identity mail, never app-defined
// network calls. Credentials and envelope keys stay outside metadata and AppIR.
package authmail

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
)

const Environment = "BEAN_AUTH_EMAIL"
const TopicPrefix = "__bean_auth."
const RequestTopic = TopicPrefix + "recovery_request"
const DeliveryTopic = TopicPrefix + "recovery_delivery"

var ErrConfiguration = errors.New("invalid auth email configuration: require SMTP address, sender, safe origin and a base64 32-byte key")
var ErrDelivery = errors.New("auth email delivery failed")
var ErrEnvelope = errors.New("invalid auth email envelope")

type Config struct {
	Address    string `json:"address"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	From       string `json:"from"`
	Origin     string `json:"origin"`
	Key        string `json:"key"`
	RootCAFile string `json:"rootCAFile"`
}
type Message struct{ To, Link string }
type Sender interface {
	Send(context.Context, Message) error
}
type Envelope struct {
	ID, AppID, ReleaseID, Email, Token string
	Expires                            time.Time
}
type Service struct {
	aead   cipher.AEAD
	origin string
	sender Sender
}

func FromEnvironment(raw string) (*Service, error) {
	if raw == "" {
		return nil, nil
	}
	var config Config
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&config) != nil {
		return nil, ErrConfiguration
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, ErrConfiguration
	}
	return New(config, nil)
}

// New accepts an injected sender for embedding/tests, but applies the same
// origin, key and envelope validation as production delivery.
func New(config Config, sender Sender) (*Service, error) {
	host, port, err := net.SplitHostPort(config.Address)
	if err != nil || host == "" || port == "" || !ValidEmail(config.From) {
		return nil, ErrConfiguration
	}
	if (config.Username == "") != (config.Password == "") {
		return nil, ErrConfiguration
	}
	origin, err := url.Parse(config.Origin)
	if err != nil || origin.Hostname() == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
		return nil, ErrConfiguration
	}
	local := origin.Hostname() == "localhost" || origin.Hostname() == "127.0.0.1" || origin.Hostname() == "::1"
	if origin.Scheme != "https" && !(origin.Scheme == "http" && local) {
		return nil, ErrConfiguration
	}
	key, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil || len(key) != 32 {
		return nil, ErrConfiguration
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrConfiguration
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrConfiguration
	}
	if sender == nil {
		var roots *x509.CertPool
		if config.RootCAFile != "" {
			file, err := os.Open(config.RootCAFile)
			if err != nil {
				return nil, ErrConfiguration
			}
			pem, err := io.ReadAll(io.LimitReader(file, 1<<20))
			file.Close()
			if err != nil {
				return nil, ErrConfiguration
			}
			roots, _ = x509.SystemCertPool()
			if roots == nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM(pem) {
				return nil, ErrConfiguration
			}
		}
		sender = &smtpSender{config: config, host: host, roots: roots}
	}
	return &Service{aead: aead, origin: strings.TrimSuffix(config.Origin, "/"), sender: sender}, nil
}
func ValidEmail(value string) bool {
	if len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value && parsed.Name == ""
}
func (s *Service) Seal(topic string, message Envelope) (map[string]any, error) {
	if s == nil {
		return nil, ErrConfiguration
	}
	plaintext, err := json.Marshal(message)
	if err != nil {
		return nil, ErrEnvelope
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, ErrEnvelope
	}
	sealed := s.aead.Seal(nonce, nonce, plaintext, []byte(topic))
	return map[string]any{"sealed": base64.RawURLEncoding.EncodeToString(sealed)}, nil
}
func (s *Service) Open(topic string, payload map[string]any) (Envelope, error) {
	if s == nil {
		return Envelope{}, ErrConfiguration
	}
	encoded, ok := payload["sealed"].(string)
	if !ok || len(encoded) > 8192 {
		return Envelope{}, ErrEnvelope
	}
	sealed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(sealed) < s.aead.NonceSize() {
		return Envelope{}, ErrEnvelope
	}
	plaintext, err := s.aead.Open(nil, sealed[:s.aead.NonceSize()], sealed[s.aead.NonceSize():], []byte(topic))
	if err != nil {
		return Envelope{}, ErrEnvelope
	}
	var message Envelope
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&message) != nil || !ValidEmail(message.Email) || message.ID == "" || message.AppID == "" || message.ReleaseID == "" {
		return Envelope{}, ErrEnvelope
	}
	return message, nil
}
func (s *Service) Send(ctx context.Context, message Envelope) error {
	if !message.Expires.After(time.Now()) {
		return nil
	}
	if len(message.Token) != 43 {
		return ErrEnvelope
	}
	// Fragment credentials never reach HTTP request paths or Referer headers.
	link := s.origin + "/login?recovery=reset#token=" + url.QueryEscape(message.Token)
	if err := s.sender.Send(ctx, Message{To: message.Email, Link: link}); err != nil {
		return ErrDelivery
	}
	return nil
}
