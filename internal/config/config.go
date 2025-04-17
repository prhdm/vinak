package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	RedisURL           string
	RedisPassword      string
	JWTSecret          string
	SMTPHost           string
	SMTPPort           string
	SMTPUser           string
	SMTPPass           string
	ZarinpalMerchantID string
	ZarinpalSandbox    bool
	TelegramToken      string
	TelegramChatID     int64
	ServerPort         string
	NowPaymentsAPIKey  string
	PayPalClientID     string
	PayPalClientSecret string
	PayPalMode         string
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	chatID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
	if err != nil {
		return nil, err
	}

	config := &Config{
		DBHost:             os.Getenv("DB_HOST"),
		DBPort:             os.Getenv("DB_PORT"),
		DBUser:             os.Getenv("DB_USER"),
		DBPassword:         os.Getenv("DB_PASSWORD"),
		DBName:             os.Getenv("DB_NAME"),
		RedisURL:           os.Getenv("REDIS_URL"),
		RedisPassword:      os.Getenv("REDIS_PASSWORD"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		SMTPHost:           os.Getenv("SMTP_HOST"),
		SMTPPort:           os.Getenv("SMTP_PORT"),
		SMTPUser:           os.Getenv("SMTP_USER"),
		SMTPPass:           os.Getenv("SMTP_PASS"),
		ZarinpalMerchantID: os.Getenv("ZARINPAL_MERCHANT_ID"),
		ZarinpalSandbox:    os.Getenv("ZARINPAL_SANDBOX") == "true",
		TelegramToken:      os.Getenv("TELEGRAM_TOKEN"),
		TelegramChatID:     chatID,
		ServerPort:         os.Getenv("SERVER_PORT"),
		NowPaymentsAPIKey:  os.Getenv("NOWPAYMENTS_API_KEY"),
		PayPalClientID:     os.Getenv("PAYPAL_CLIENT_ID"),
		PayPalClientSecret: os.Getenv("PAYPAL_CLIENT_SECRET"),
		PayPalMode:         os.Getenv("PAYPAL_MODE"),
	}

	return config, nil
}
