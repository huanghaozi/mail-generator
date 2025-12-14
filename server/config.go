package main

import (
	"os"
)

type Config struct {
	Port            string
	SMTPPort        string
	Password        string
	DBFile          string
	JWTSecret       string
	SMTPRelayHost   string // e.g. "smtp.gmail.com" or "127.0.0.1"
	SMTPRelayPort   string // e.g. "587"
	SMTPRelayUser   string
	SMTPRelayPass   string
	DefaultEnvelope string // Address to use as MAIL FROM if needed to pass SPF
}

func LoadConfig() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		SMTPPort:        getEnv("SMTP_PORT", "2525"),
		Password:        getEnv("PASSWORD", "admin123"),
		DBFile:          getEnv("DB_FILE", "mail.db"),
		JWTSecret:       getEnv("JWT_SECRET", "very-secret-key"),
		SMTPRelayHost:   getEnv("SMTP_RELAY_HOST", ""), // Empty means direct delivery (not implemented, safer to use relay) or dry-run
		SMTPRelayPort:   getEnv("SMTP_RELAY_PORT", "587"),
		SMTPRelayUser:   getEnv("SMTP_RELAY_USER", ""),
		SMTPRelayPass:   getEnv("SMTP_RELAY_PASS", ""),
		DefaultEnvelope: getEnv("DEFAULT_ENVELOPE", "postmaster@localhost"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
