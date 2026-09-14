package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	SessionKey   string
	DocumentDir  string
	SecureCookie bool
}

func Load() (Config, error) {
	c := Config{
		Addr:         os.Getenv("MGP_ADDR"),
		DatabaseURL:  os.Getenv("MGP_DATABASE_URL"),
		SessionKey:   os.Getenv("MGP_SESSION_KEY"),
		DocumentDir:  os.Getenv("MGP_DOCUMENT_DIR"),
		SecureCookie: true,
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.DocumentDir == "" {
		c.DocumentDir = "data/documents"
	}
	if value := os.Getenv("MGP_SECURE_COOKIE"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("MGP_SECURE_COOKIE: %w", err)
		}
		c.SecureCookie = parsed
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("MGP_DATABASE_URL is required")
	}
	if len(c.SessionKey) < 32 {
		return Config{}, fmt.Errorf("MGP_SESSION_KEY must be at least 32 characters")
	}
	return c, nil
}
