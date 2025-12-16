package main

import (
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Domain represents a managed domain (for verification/instruction purposes)
type Domain struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"uniqueIndex;not null" json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Account represents an email account or forwarding rule
type Account struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Pattern     string         `gorm:"uniqueIndex;not null" json:"pattern"` // Regex or wildcards like *@domain.com
	ForwardTo   string         `gorm:"not null" json:"forward_to"`          // Target email(s), comma separated
	Description string         `json:"description"`
	HitCount    int64          `gorm:"default:0" json:"hit_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Log represents a forwarding log
type Log struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	From      string    `gorm:"index" json:"from"`
	To        string    `gorm:"index" json:"to"`
	Subject   string    `json:"subject"`
	Content   string    `json:"content"`    // Decoded text/plain content (truncated)
	Raw       string    `json:"raw"`        // Raw RFC822 content (truncated)
	Status    string    `json:"status"`  // "success", "failed"
	Error     string    `json:"error,omitempty"`
	ClientIP  string    `json:"client_ip"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

var DB *gorm.DB

func InitDB(cfg *Config) {
	var err error
	DB, err = gorm.Open(sqlite.Open(cfg.DBFile), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}

	// Migrate the schema
	err = DB.AutoMigrate(&Domain{}, &Account{}, &Log{})
	if err != nil {
		panic("failed to migrate database: " + err.Error())
	}
}
