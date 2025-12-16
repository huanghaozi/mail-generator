package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"regexp"
	"sort"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
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
	parts := strings.Split(to, "@")
	if len(parts) != 2 {
		return errors.New("invalid address")
	}

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
	rawData := buf.String()

	// Parse the email
	subject := extractSubject(rawData)
	textBody := extractTextBody(rawData)

	// Decode subject if encoded
	decodedSubject := decodeRFC2047(subject)

	// Limit content length for DB
	contentToLog := textBody
	if len(contentToLog) > 10000 {
		contentToLog = contentToLog[:10000] + "...(truncated)"
	}

	logEntry := Log{
		From:      s.From,
		To:        s.To,
		Subject:   decodedSubject,
		Content:   contentToLog,
		Status:    "processing",
		ClientIP:  "",
		CreatedAt: time.Now(),
	}
	DB.Create(&logEntry)

	// Forward asynchronously
	go func(l Log, rule Account, originalFrom string, decodedSubj string, body string, cfg *Config) {
		var err error
		if cfg.SMTPRelayHost != "" {
			err = forwardEmailViaRelay(cfg, rule.ForwardTo, originalFrom, decodedSubj, body)
		} else {
			err = forwardEmailDirectly(cfg, rule.ForwardTo, originalFrom, decodedSubj, body)
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

		DB.Model(&l).Updates(map[string]interface{}{
			"status": status,
			"error":  errMsg,
		})

		DB.Model(&rule).Update("hit_count", rule.HitCount+1)
	}(logEntry, *s.Rule, s.From, decodedSubject, textBody, s.Config)

	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

// decodeRFC2047 decodes RFC 2047 encoded-word (e.g. =?gb2312?B?...?=)
func decodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	dec.CharsetReader = charsetReader
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// charsetReader returns a reader that converts charset to UTF-8
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	charset = strings.ToLower(charset)
	switch charset {
	case "gb2312", "gbk", "gb18030":
		return transform.NewReader(input, simplifiedchinese.GBK.NewDecoder()), nil
	default:
		return input, nil
	}
}

func extractSubject(body string) string {
	headerEnd := strings.Index(body, "\r\n\r\n")
	if headerEnd == -1 {
		headerEnd = len(body)
	}
	headers := body[:headerEnd]

	lines := strings.Split(headers, "\r\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "subject:") {
			subject := strings.TrimSpace(line[8:])
			// Handle folded headers
			for j := i + 1; j < len(lines); j++ {
				if len(lines[j]) > 0 && (lines[j][0] == ' ' || lines[j][0] == '\t') {
					subject += strings.TrimSpace(lines[j])
				} else {
					break
				}
			}
			return subject
		}
	}
	return ""
}

func extractTextBody(rawEmail string) string {
	// Split headers and body
	parts := strings.SplitN(rawEmail, "\r\n\r\n", 2)
	if len(parts) < 2 {
		return ""
	}
	headers := parts[0]
	body := parts[1]

	// Get Content-Type
	contentType := ""
	boundary := ""
	transferEncoding := ""
	charset := "utf-8"

	headerLines := strings.Split(headers, "\r\n")
	for i, line := range headerLines {
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "content-type:") {
			contentType = line[13:]
			// Handle folded headers
			for j := i + 1; j < len(headerLines); j++ {
				if len(headerLines[j]) > 0 && (headerLines[j][0] == ' ' || headerLines[j][0] == '\t') {
					contentType += headerLines[j]
				} else {
					break
				}
			}
		}
		if strings.HasPrefix(lowerLine, "content-transfer-encoding:") {
			transferEncoding = strings.TrimSpace(line[26:])
		}
	}

	// Parse Content-Type
	if contentType != "" {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
		if err == nil {
			if b, ok := params["boundary"]; ok {
				boundary = b
			}
			if c, ok := params["charset"]; ok {
				charset = c
			}

			// If multipart, extract text/plain part
			if strings.HasPrefix(mediaType, "multipart/") && boundary != "" {
				return extractFromMultipart(body, boundary)
			}
		}
	}

	// Single part message - decode it
	return decodeBody(body, transferEncoding, charset)
}

func extractFromMultipart(body string, boundary string) string {
	reader := multipart.NewReader(strings.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}

		partContentType := part.Header.Get("Content-Type")
		partEncoding := part.Header.Get("Content-Transfer-Encoding")

		mediaType, params, _ := mime.ParseMediaType(partContentType)
		charset := "utf-8"
		if c, ok := params["charset"]; ok {
			charset = c
		}

		// Prefer text/plain
		if strings.HasPrefix(mediaType, "text/plain") {
			content, _ := io.ReadAll(part)
			return decodeBody(string(content), partEncoding, charset)
		}

		// If nested multipart, recurse
		if strings.HasPrefix(mediaType, "multipart/") {
			if b, ok := params["boundary"]; ok {
				content, _ := io.ReadAll(part)
				result := extractFromMultipart(string(content), b)
				if result != "" {
					return result
				}
			}
		}
	}

	// If no text/plain found, try text/html as fallback
	reader = multipart.NewReader(strings.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}

		partContentType := part.Header.Get("Content-Type")
		partEncoding := part.Header.Get("Content-Transfer-Encoding")

		mediaType, params, _ := mime.ParseMediaType(partContentType)
		charset := "utf-8"
		if c, ok := params["charset"]; ok {
			charset = c
		}

		if strings.HasPrefix(mediaType, "text/html") {
			content, _ := io.ReadAll(part)
			decoded := decodeBody(string(content), partEncoding, charset)
			// Strip HTML tags (simple)
			return stripHTML(decoded)
		}
	}

	return ""
}

func decodeBody(body string, encoding string, charset string) string {
	var decoded []byte
	var err error

	encoding = strings.ToLower(strings.TrimSpace(encoding))

	switch encoding {
	case "base64":
		decoded, err = base64.StdEncoding.DecodeString(strings.ReplaceAll(body, "\r\n", ""))
		if err != nil {
			decoded = []byte(body)
		}
	case "quoted-printable":
		reader := quotedprintable.NewReader(strings.NewReader(body))
		decoded, err = io.ReadAll(reader)
		if err != nil {
			decoded = []byte(body)
		}
	default:
		decoded = []byte(body)
	}

	// Convert charset to UTF-8
	charset = strings.ToLower(charset)
	switch charset {
	case "gb2312", "gbk", "gb18030":
		utf8Bytes, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), decoded)
		if err == nil {
			return string(utf8Bytes)
		}
	}

	return string(decoded)
}

func stripHTML(s string) string {
	// Simple HTML tag removal
	re := regexp.MustCompile(`<[^>]*>`)
	result := re.ReplaceAllString(s, "")
	// Decode common HTML entities
	// NOTE: &amp; MUST be replaced FIRST so nested entities like &amp;nbsp; decode correctly
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	return strings.TrimSpace(result)
}

// forwardEmailViaRelay sends email using a configured SMTP relay (e.g. 163.com)
func forwardEmailViaRelay(cfg *Config, to string, originalFrom string, subject string, textBody string) error {
	addr := fmt.Sprintf("%s:%s", cfg.SMTPRelayHost, cfg.SMTPRelayPort)

	envelopeFrom := cfg.SMTPRelayUser
	if envelopeFrom == "" {
		envelopeFrom = cfg.DefaultEnvelope
	}

	// Build a clean, simple email
	newSubject := fmt.Sprintf("[Fwd: %s] %s", originalFrom, subject)
	if len(newSubject) > 78 { // RFC 2822 recommends max 78 chars for Subject
		newSubject = newSubject[:75] + "..."
	}

	var fullMsg bytes.Buffer
	fullMsg.WriteString(fmt.Sprintf("From: %s\r\n", envelopeFrom))
	fullMsg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	fullMsg.WriteString(fmt.Sprintf("Subject: %s\r\n", newSubject))
	fullMsg.WriteString("MIME-Version: 1.0\r\n")
	fullMsg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	fullMsg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	fullMsg.WriteString("\r\n")
	fullMsg.WriteString(fmt.Sprintf("Original Sender: %s\r\n", originalFrom))
	fullMsg.WriteString(fmt.Sprintf("Original Subject: %s\r\n", subject))
	fullMsg.WriteString("---\r\n\r\n")
	fullMsg.WriteString(textBody)

	// Use low-level SMTP client for better control and debugging
	log.Printf("[Relay] Connecting to %s...", addr)

	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return fmt.Errorf("dial failed: %v", err)
	}

	c, err := smtp.NewClient(conn, cfg.SMTPRelayHost)
	if err != nil {
		conn.Close()
		return fmt.Errorf("new client failed: %v", err)
	}
	defer c.Close()

	// Say hello
	if err := c.Hello("localhost"); err != nil {
		return fmt.Errorf("HELO failed: %v", err)
	}

	// STARTTLS if available (required by 163.com on port 25)
	if ok, _ := c.Extension("STARTTLS"); ok {
		log.Printf("[Relay] Starting TLS...")
		tlsConfig := &tls.Config{
			ServerName:         cfg.SMTPRelayHost,
			InsecureSkipVerify: false,
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			log.Printf("[Relay] STARTTLS failed: %v, trying without TLS", err)
		}
	}

	// Authenticate
	if cfg.SMTPRelayUser != "" && cfg.SMTPRelayPass != "" {
		log.Printf("[Relay] Authenticating as %s...", cfg.SMTPRelayUser)
		auth := smtp.PlainAuth("", cfg.SMTPRelayUser, cfg.SMTPRelayPass, cfg.SMTPRelayHost)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("auth failed: %v", err)
		}
	}

	// MAIL FROM
	log.Printf("[Relay] MAIL FROM: %s", envelopeFrom)
	if err := c.Mail(envelopeFrom); err != nil {
		return fmt.Errorf("MAIL FROM failed: %v", err)
	}

	// RCPT TO
	log.Printf("[Relay] RCPT TO: %s", to)
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %v", err)
	}

	// DATA
	log.Printf("[Relay] Sending DATA...")
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %v", err)
	}

	msgBytes := fullMsg.Bytes()
	log.Printf("[Relay] Writing %d bytes...", len(msgBytes))

	_, err = wc.Write(msgBytes)
	if err != nil {
		wc.Close()
		return fmt.Errorf("write data failed: %v", err)
	}

	if err = wc.Close(); err != nil {
		return fmt.Errorf("close data failed: %v", err)
	}

	log.Printf("[Relay] Sending QUIT...")
	c.Quit()

	log.Printf("[Relay] Email sent successfully")
	return nil
}

// forwardEmailDirectly looks up MX records and delivers mail directly
func forwardEmailDirectly(cfg *Config, to string, originalFrom string, subject string, textBody string) error {
	parts := strings.Split(to, "@")
	if len(parts) != 2 {
		return errors.New("invalid to address")
	}
	domain := parts[1]

	mxs, err := net.LookupMX(domain)
	if err != nil {
		return fmt.Errorf("mx lookup failed: %v", err)
	}
	if len(mxs) == 0 {
		return errors.New("no mx records found")
	}

	sort.Slice(mxs, func(i, j int) bool {
		return mxs[i].Pref < mxs[j].Pref
	})

	// Build message
	newSubject := fmt.Sprintf("[Fwd: %s] %s", originalFrom, subject)
	if len(newSubject) > 200 {
		newSubject = newSubject[:197] + "..."
	}

	var fullMsg bytes.Buffer
	fullMsg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.DefaultEnvelope))
	fullMsg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	fullMsg.WriteString(fmt.Sprintf("Subject: %s\r\n", newSubject))
	fullMsg.WriteString("MIME-Version: 1.0\r\n")
	fullMsg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	fullMsg.WriteString("\r\n")

	fullMsg.WriteString(fmt.Sprintf("Original Sender: %s\r\n", originalFrom))
	fullMsg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	fullMsg.WriteString("---\r\n\r\n")
	fullMsg.WriteString(textBody)

	var lastErr error
	for _, mx := range mxs {
		address := mx.Host + ":25"
		log.Printf("Attempting direct delivery to %s (%s)", address, to)

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
			continue
		}

		if err := c.Mail(cfg.DefaultEnvelope); err != nil {
			lastErr = err
			c.Close()
			continue
		}

		if err := c.Rcpt(to); err != nil {
			lastErr = err
			c.Close()
			continue
		}

		wc, err := c.Data()
		if err != nil {
			lastErr = err
			c.Close()
			continue
		}

		_, err = wc.Write(fullMsg.Bytes())
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
		return nil
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
	s.MaxMessageBytes = 1024 * 1024 * 10
	s.AllowInsecureAuth = true

	log.Printf("Starting SMTP server on %s", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
