package main

import (
	"ak47/internal/config"
	"ak47/internal/handlers"
	"ak47/internal/models"
	"ak47/pkg/payment"
	"ak47/pkg/telegram"
	"context"
	"log"

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

	if err := db.AutoMigrate(&models.User{}, &models.Payment{}, &models.PaymentLog{}, &models.UserPayment{}); err != nil {
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
	zarinpalService := payment.NewZarinpalService(cfg.ZarinpalMerchantID, cfg.ZarinpalSandbox)

	telegramService, err := telegram.NewTelegramService(cfg.TelegramToken, cfg.TelegramChatID)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram service: %v", err)
	}

	paymentHandler := handlers.NewPaymentHandler(db, nowpaymentsService, zarinpalService, telegramService)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://127.0.0.1:5502", "https://ak47album.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
	r.GET("/api/top-users", paymentHandler.GetTopUsers)
	r.GET("/api/payments/zarinpal/callback", paymentHandler.HandleZarinpalCallback)
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
