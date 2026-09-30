package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Token          string
	StudentsFile   string
	AttendanceFile string
}

// Load reads settings from .env (if present) and the environment.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("чтение .env: %w", err)
	}

	cfg := &Config{
		Token:          os.Getenv("TELEGRAM_BOT_TOKEN"),
		StudentsFile:   getEnv("STUDENTS_FILE", "data/students.json"),
		AttendanceFile: getEnv("ATTENDANCE_FILE", "attendance.csv"),
	}
	if cfg.Token == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN не задан: создай .env по образцу .env.example")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
