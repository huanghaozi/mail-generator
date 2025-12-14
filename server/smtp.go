package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

type Backend struct {
	Config *Config
}

func (b *Backend) NewSession(c *gosmtp.Conn) (gosmtp.Session, error) {
	return &Session{Config: b.Config}, nil
}

type Session struct {
	Config *Config
	From   string
	To     string
	Rule   *Account
}

func (s *Session) AuthPlain(username, password string) error {
	return nil // Accept all incoming connection (act as MTA)
}

func (s *Session) Mail(from string, opts *gosmtp.MailOptions) error {
	s.From = from
	return nil
}

func (s *Session) Rcpt(to string, opts *gosmtp.RcptOptions) error {
	// 1. Parse domain
	parts := strings.Split(to, "@")
	if len(parts) != 2 {
		return errors.New("invalid address")
	}

	// 2. Find matching rule in DB
	var accounts []Account
	DB.Find(&accounts)

	for _, acc := range accounts {
		matched, err := regexp.MatchString(acc.Pattern, to)
		if err == nil && matched {
			s.To = to
			s.Rule = &acc
			return nil
		}
	}

	return errors.New("no relay allowed")
}

func (s *Session) Data(r io.Reader) error {
	if s.Rule == nil {
		return errors.New("no recipient")
	}

	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return err
	}
	data := buf.String()

	// Log the attempt
	logEntry := Log{
		From:      s.From,
		To:        s.To,
		Subject:   extractSubject(data),
		Status:    "processing",
		ClientIP:  "", // Context not passed easily in this lib
		CreatedAt: time.Now(),
	}
	DB.Create(&logEntry)

	// Forward asynchronously
	go func(l Log, rule Account, content string, cfg *Config) {
		err := forwardEmail(cfg, rule.ForwardTo, content)
		status := "success"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
			log.Printf("Failed to forward email: %v", err)
		}

		// Update Log
		DB.Model(&l).Updates(map[string]interface{}{
			"status": status,
			"error":  errMsg,
		})

		// Update Hit Count
		DB.Model(&rule).Update("hit_count", rule.HitCount+1)
	}(logEntry, *s.Rule, data, s.Config)

	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

func extractSubject(body string) string {
	var subject string
	// Try to find Subject header
	// Basic implementation
	headerEnd := strings.Index(body, "\r\n\r\n")
	if headerEnd == -1 {
		headerEnd = len(body)
	}
	headers := body[:headerEnd]
	
	lines := strings.Split(headers, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Subject: ") || strings.HasPrefix(line, "subject: ") {
			subject = strings.TrimSpace(line[9:])
			break
		}
	}
	return subject
}

func forwardEmail(cfg *Config, to string, body string) error {
	if cfg.SMTPRelayHost == "" {
		return errors.New("SMTP relay not configured")
	}

	// Prepare authentication
	var auth smtp.Auth
	if cfg.SMTPRelayUser != "" && cfg.SMTPRelayPass != "" {
		auth = smtp.PlainAuth("", cfg.SMTPRelayUser, cfg.SMTPRelayPass, cfg.SMTPRelayHost)
	}

	// Address to connect to
	addr := fmt.Sprintf("%s:%s", cfg.SMTPRelayHost, cfg.SMTPRelayPort)

	// Determine Envelope From
	// Using a default fixed envelope sender avoids SPF issues for simple forwarding
	// The original "From" header inside `body` remains unchanged, preserving the sender identity in the mail client.
	envelopeFrom := cfg.DefaultEnvelope 

	// Send the email
	// Note: We send the original body as-is (headers + content).
	// Ideally we might want to prepend "Resent-From" or similar, but sending raw is often fine for personal forwarding.
	err := smtp.SendMail(addr, auth, envelopeFrom, []string{to}, []byte(body))
	if err != nil {
		return err
	}

	log.Printf("Successfully forwarded email to %s via %s", to, cfg.SMTPRelayHost)
	return nil
}

func StartSMTPServer(cfg *Config) {
	be := &Backend{Config: cfg}
	s := gosmtp.NewServer(be)
	s.Addr = ":" + cfg.SMTPPort
	s.Domain = "localhost"
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = 1024 * 1024 * 10 // 10MB
	s.AllowInsecureAuth = true           // Since we don't really do auth, and often behind proxy/container

	log.Printf("Starting SMTP server on %s", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

