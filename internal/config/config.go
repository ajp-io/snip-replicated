package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	Port        string
	BaseURL     string
	SDKEndpoint string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		Port:        os.Getenv("PORT"),
		BaseURL:     os.Getenv("BASE_URL"),
		SDKEndpoint: os.Getenv("REPLICATED_SDK_ENDPOINT"),
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.SDKEndpoint == "" {
		c.SDKEndpoint = "http://snip-sdk:3000"
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return Config{}, errors.New("REDIS_URL is required")
	}
	if c.BaseURL == "" {
		return Config{}, errors.New("BASE_URL is required")
	}
	return c, nil
}
