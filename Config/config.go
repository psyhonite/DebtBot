package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken string
	DBName   string // Теперь DBName будет путем к файлу SQLite
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	return &Config{
		BotToken: os.Getenv("BOT_TOKEN"),
		DBName:   os.Getenv("DB_NAME"), // DB_NAME теперь путь к файлу SQLite
	}
}
