package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	Seed        bool
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		Port:        os.Getenv("PORT"),
		Seed:        os.Getenv("SEED") == "1" || os.Getenv("SEED") == "true",
	}
	if c.Port == "" {
		c.Port = "8086"
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

func (c Config) Validate() error {
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < 16 {
		return errors.New("JWT_SECRET must be at least 16 characters")
	}
	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		return errors.New("PORT must be a valid port number")
	}
	return nil
}
