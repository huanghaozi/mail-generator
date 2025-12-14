package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"regexp"
	"sort"
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
		var err error
		if cfg.SMTPRelayHost != "" {
			// Relay Mode
			err = forwardEmailViaRelay(cfg, rule.ForwardTo, content)
		} else {
			// Direct Send Mode
			err = forwardEmailDirectly(cfg, rule.ForwardTo, content)
		}

		status := "success"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
			log.Printf("Failed to forward email to %s: %v", rule.ForwardTo, err)
		} else {
			log.Printf("Successfully forwarded email to %s", rule.ForwardTo)
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

// forwardEmailViaRelay sends email using a configured SMTP relay (e.g. Gmail)
func forwardEmailViaRelay(cfg *Config, to string, body string) error {
	// Prepare authentication
	var auth smtp.Auth
	if cfg.SMTPRelayUser != "" && cfg.SMTPRelayPass != "" {
		auth = smtp.PlainAuth("", cfg.SMTPRelayUser, cfg.SMTPRelayPass, cfg.SMTPRelayHost)
	}

	// Address to connect to
	addr := fmt.Sprintf("%s:%s", cfg.SMTPRelayHost, cfg.SMTPRelayPort)
	
	// When using relay, we MUST use the auth user as Envelope From (MAIL FROM)
	// Many SMTP providers (like 163, Gmail) require MAIL FROM == Auth User
	envelopeFrom := cfg.SMTPRelayUser
	if envelopeFrom == "" {
		envelopeFrom = cfg.DefaultEnvelope
	}

	// Rewrite headers to avoid "550 Header mismatch" errors
	// 1. Parse existing headers
	headerEnd := strings.Index(body, "\r\n\r\n")
	if headerEnd == -1 {
		// Fallback for simple bodies
		headerEnd = 0
	}
	
	originalHeaders := body[:headerEnd]
	originalBody := body[headerEnd:]
	
	// 2. Extract original From to use as Reply-To
	var originalFrom string
	lines := strings.Split(originalHeaders, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "From: ") {
			originalFrom = strings.TrimSpace(line[6:])
			break
		}
	}

	// 3. Construct new headers
	// We replace the From header to match our relay account, but keep Reply-To pointing to original sender
	var newHeaders bytes.Buffer
	newHeaders.WriteString(fmt.Sprintf("From: %s\r\n", envelopeFrom))
	newHeaders.WriteString(fmt.Sprintf("To: %s\r\n", to))
	if originalFrom != "" {
		newHeaders.WriteString(fmt.Sprintf("Reply-To: %s\r\n", originalFrom))
	}
	// Add other critical headers like Subject
	for _, line := range lines {
		if strings.HasPrefix(line, "Subject:") || strings.HasPrefix(line, "Date:") || strings.HasPrefix(line, "MIME-Version:") || strings.HasPrefix(line, "Content-Type:") {
			newHeaders.WriteString(line + "\r\n")
		}
	}
	newHeaders.WriteString("\r\n") // End of headers
	
	// Combine new headers with original body
	finalBody := newHeaders.Bytes()
	finalBody = append(finalBody, []byte(originalBody)...)

	return smtp.SendMail(addr, auth, envelopeFrom, []string{to}, finalBody)
}

// forwardEmailDirectly looks up MX records and delivers mail directly
func forwardEmailDirectly(cfg *Config, to string, body string) error {
	parts := strings.Split(to, "@")
	if len(parts) != 2 {
		return errors.New("invalid to address")
	}
	domain := parts[1]

	// 1. Lookup MX records
	mxs, err := net.LookupMX(domain)
	if err != nil {
		return fmt.Errorf("mx lookup failed: %v", err)
	}
	if len(mxs) == 0 {
		return errors.New("no mx records found")
	}

	// Sort by preference
	sort.Slice(mxs, func(i, j int) bool {
		return mxs[i].Pref < mxs[j].Pref
	})

	// 2. Try each MX record
	var lastErr error
	for _, mx := range mxs {
		address := mx.Host + ":25"
		log.Printf("Attempting direct delivery to %s (%s)", address, to)
		
		// Connect to the remote SMTP server with timeout
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			lastErr = err
			log.Printf("Failed to connect to %s: %v", address, err)
			continue
		}

		c, err := smtp.NewClient(conn, mx.Host)
		if err != nil {
			conn.Close()
			lastErr = err
			log.Printf("Failed to create SMTP client for %s: %v", address, err)
			continue
		}
		
		// Set the sender (Envelope From)
		if err := c.Mail(cfg.DefaultEnvelope); err != nil {
			lastErr = err
			c.Close()
			continue
		}
		
		// Set the recipient
		if err := c.Rcpt(to); err != nil {
			lastErr = err
			c.Close()
			continue
		}
		
		// Send the data
		wc, err := c.Data()
		if err != nil {
			lastErr = err
			c.Close()
			continue
		}
		
		_, err = wc.Write([]byte(body))
		if err != nil {
			lastErr = err
			wc.Close()
			c.Close()
			continue
		}
		
		err = wc.Close()
		if err != nil {
			lastErr = err
			c.Close()
			continue
		}
		
		c.Quit()
		return nil // Success
	}

	if lastErr != nil {
		return fmt.Errorf("all mx servers failed, last error: %v", lastErr)
	}
	return errors.New("delivery failed")
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
