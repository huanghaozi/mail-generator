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
	// Limit content length for DB
	contentToLog := data
	if len(contentToLog) > 10000 {
		contentToLog = contentToLog[:10000] + "...(truncated)"
	}

	logEntry := Log{
		From:      s.From,
		To:        s.To,
		Subject:   extractSubject(data),
		Content:   contentToLog,
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
	envelopeFrom := cfg.SMTPRelayUser
	if envelopeFrom == "" {
		envelopeFrom = cfg.DefaultEnvelope
	}

	// --- Construct a completely new, safe email ---

	// 1. Parse simple headers for Subject/From display
	headerEnd := strings.Index(body, "\r\n\r\n")
	if headerEnd == -1 {
		headerEnd = 0
	}
	originalHeaders := body[:headerEnd]

	var originalFrom, originalSubject string
	lines := strings.Split(originalHeaders, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "From: ") {
			originalFrom = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "Subject: ") {
			originalSubject = strings.TrimSpace(line[9:])
		}
	}
	if originalFrom == "" {
		originalFrom = "Unknown Sender"
	}
	if originalSubject == "" {
		originalSubject = "No Subject"
	}

	// 2. Format new Subject
	newSubject := fmt.Sprintf("[Fwd: %s] %s", originalFrom, originalSubject)
	if len(newSubject) > 200 {
		newSubject = newSubject[:197] + "..."
	}

	// 3. Construct Body
	// To safely include ANY original content (binary, html, attachments),
	// we will wrap the *entire* original raw message as a text/plain attachment
	// or just dump it inside but safe from protocol injection.
	// Actually, best user experience for "viewing" is tricky without parsing.
	// Let's just include a safe plain text notice + the raw content in a safe way.

	var fullMsg bytes.Buffer
	fullMsg.WriteString(fmt.Sprintf("From: %s\r\n", envelopeFrom))
	fullMsg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	fullMsg.WriteString(fmt.Sprintf("Subject: %s\r\n", newSubject))
	fullMsg.WriteString("MIME-Version: 1.0\r\n")
	fullMsg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	fullMsg.WriteString("\r\n") // End of Headers

	fullMsg.WriteString("--- Forwarded Message Info ---\r\n")
	fullMsg.WriteString(fmt.Sprintf("Original Sender: %s\r\n", originalFrom))
	fullMsg.WriteString(fmt.Sprintf("Original Subject: %s\r\n", originalSubject))
	fullMsg.WriteString("------------------------------\r\n")
	fullMsg.WriteString("\r\n")
	fullMsg.WriteString("(The original email content may be complex HTML or contain attachments.\r\n")
	fullMsg.WriteString(" Please check the Web Dashboard to view the full raw content correctly.)\r\n")
	fullMsg.WriteString("\r\n")
	fullMsg.WriteString("--- Raw Content Snippet (First 500 chars) ---\r\n")

	// Safely grab a snippet
	snippetLen := 500
	if len(body) < 500 {
		snippetLen = len(body)
	}
	// Sanitize snippet to prevent header injection if we were to act on it,
	// but here it is just body text.
	fullMsg.WriteString(body[:snippetLen])
	fullMsg.WriteString("\r\n...\r\n")

	return smtp.SendMail(addr, auth, envelopeFrom, []string{to}, fullMsg.Bytes())
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
