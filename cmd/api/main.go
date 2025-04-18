package main

import (
	"context"
	"log"
	"vinak/internal/config"
	"vinak/internal/handlers"
	"vinak/internal/models"
	"vinak/pkg/payment"
	"vinak/pkg/telegram"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dsn := "host=" + cfg.DBHost + " user=" + cfg.DBUser + " password=" + cfg.DBPassword +
		" dbname=" + cfg.DBName + " port=" + cfg.DBPort + " sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Payment{}, &models.PaymentLog{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
	})
	_, err = rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize payment services
	nowpaymentsService := payment.NewNowPaymentsService(cfg.NowPaymentsAPIKey)
	paypalService, err := payment.NewPayPalService(cfg.PayPalClientID, cfg.PayPalClientSecret, cfg.PayPalMode)
	if err != nil {
		log.Fatalf("Failed to initialize PayPal service: %v", err)
	}
	zarinpalService := payment.NewZarinpalService(cfg.ZarinpalMerchantID, cfg.ZarinpalSandbox)

	// Initialize Telegram service
	telegramService, err := telegram.NewTelegramService(cfg.TelegramToken, cfg.TelegramChatID)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram service: %v", err)
	}

	paymentHandler := handlers.NewPaymentHandler(db, nowpaymentsService, paypalService, zarinpalService, telegramService)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://127.0.0.1:5502", "https://ak47album.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
	r.POST("/api/payments", paymentHandler.CreatePayment)
	r.GET("/api/top-users", paymentHandler.GetTopUsers)
	r.POST("/api/payments/paypal/callback", paymentHandler.HandlePayPalCallback)
	r.POST("/api/payments/zarinpal/callback", paymentHandler.HandleZarinpalCallback)
	r.POST("/api/payments/nowpayments/callback", paymentHandler.HandleNowPaymentsCallback)
	r.POST("/api/payment/prepare", paymentHandler.PreparePayment)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
