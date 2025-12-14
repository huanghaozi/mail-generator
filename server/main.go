package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := LoadConfig()

	// Initialize Database
	InitDB(cfg)

	// Start SMTP Server in background
	go StartSMTPServer(cfg)

	// Setup Web Server
	r := gin.Default()

	// CORS (Simple for now)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.POST("/api/login", RateLimitMiddleware(), LoginHandler(cfg))

	authorized := r.Group("/api")
	authorized.Use(AuthMiddleware(cfg))
	{
		// Domains
		authorized.GET("/domains", GetDomains)
		authorized.POST("/domains", CreateDomain)
		authorized.DELETE("/domains/:id", DeleteDomain)

		// Accounts
		authorized.GET("/accounts", GetAccounts)
		authorized.POST("/accounts", CreateAccount)
		authorized.PUT("/accounts/:id", UpdateAccount)
		authorized.DELETE("/accounts/:id", DeleteAccount)

		// Logs
		authorized.GET("/logs", GetLogs)
	}

	r.Run(":" + cfg.Port)
}
