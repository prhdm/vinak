package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"ak47/internal/models"
	"ak47/pkg/payment"
	"ak47/pkg/telegram"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PaymentHandler struct {
	db                 *gorm.DB
	nowpaymentsService *payment.NowPaymentsService
	zarinpalService    *payment.ZarinpalService
	telegramService    *telegram.TelegramService
}

func NewPaymentHandler(db *gorm.DB, nowpaymentsService *payment.NowPaymentsService, zarinpalService *payment.ZarinpalService, telegramService *telegram.TelegramService) *PaymentHandler {
	return &PaymentHandler{
		db:                 db,
		nowpaymentsService: nowpaymentsService,
		zarinpalService:    zarinpalService,
		telegramService:    telegramService,
	}
}

type PreparePaymentRequest struct {
	Name        string  `json:"name" binding:"required"`
	InstagramID string  `json:"instagram_id" binding:"required"`
	Email       string  `json:"email" binding:"required,email"`
	Currency    string  `json:"currency" binding:"required,oneof=usd irr btc"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	AuthorityID string  `json:"authority_id" binding:"required"`
}

func (h *PaymentHandler) HandleNowPaymentsCallback(c *gin.Context) {
	var callback struct {
		PaymentID     string  `json:"payment_id"`
		PaymentStatus string  `json:"payment_status"`
		PayAmount     float64 `json:"pay_amount"`
		PayCurrency   string  `json:"pay_currency"`
	}

	if err := c.ShouldBindJSON(&callback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Query the PaymentLog table using JSONB query to find the payment ID
	var paymentLog models.PaymentLog
	if err := h.db.Where("data->>'payment_id' = ?", callback.PaymentID).First(&paymentLog).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment log not found"})
		return
	}

	// Retrieve the user ID from the payment log
	var user models.User
	if err := h.db.Where("id = ?", paymentLog.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find user"})
		return
	}

	// Verify the payment with NowPayments
	if callback.PaymentStatus != "finished" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Payment verification failed"})
		return
	}

	// Update or create user payment record
	var userPayment models.UserPayment
	if err := h.db.Where("user_id = ?", user.ID).First(&userPayment).Error; err != nil {
		// If not found, create a new record
		userPayment = models.UserPayment{
			UserID: user.ID,
			Amount: callback.PayAmount / 100000,
		}
		if err := h.db.Create(&userPayment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user payment record"})
			return
		}
	} else {
		// If found, update the amount
		userPayment.Amount += callback.PayAmount / 100000
		if err := h.db.Save(&userPayment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user payment record"})
			return
		}
	}

	// Create payment log with user ID
	newPaymentLog := models.PaymentLog{
		PaymentID: paymentLog.PaymentID,
		Event:     "nowpayments_payment_completed",
		Data:      fmt.Sprintf(`{"payment_id": "%s", "status": "%s", "user_id": %d}`, callback.PaymentID, callback.PaymentStatus, user.ID),
	}

	if err := h.db.Create(&newPaymentLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment log"})
		return
	}

	// Send Telegram notification
	if err := h.telegramService.SendPaymentNotification(
		user.Name,
		user.InstagramID,
		callback.PayAmount,
		callback.PayCurrency,
		time.Now(),
	); err != nil {
		log.Printf("Failed to send Telegram notification: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"payment_id": callback.PaymentID,
		"amount":     callback.PayAmount,
	})
}

func (h *PaymentHandler) GetTopUsers(c *gin.Context) {
	var topUsers []struct {
		Name        string  `json:"name"`
		InstagramID string  `json:"instagram_id"`
		TotalAmount float64 `json:"total_amount"`
	}

	err := h.db.Model(&models.UserPayment{}).
		Select("users.name, users.instagram_id, user_payments.amount as total_amount").
		Joins("JOIN users ON users.id = user_payments.user_id").
		Order("total_amount DESC").
		Limit(10).
		Scan(&topUsers).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve top users"})
		return
	}

	// Initialize empty array if no users found
	if topUsers == nil {
		topUsers = make([]struct {
			Name        string  `json:"name"`
			InstagramID string  `json:"instagram_id"`
			TotalAmount float64 `json:"total_amount"`
		}, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"top_users": topUsers,
	})
}

func (h *PaymentHandler) HandleZarinpalCallback(c *gin.Context) {
	authority := c.Query("Authority")
	status := c.Query("Status")

	if authority == "" || status != "OK" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payment was canceled or invalid"})
		return
	}

	// Query the PaymentLog table using JSONB query to find the authority ID
	var paymentLog models.PaymentLog
	if err := h.db.Where("data->>'authority_id' = ?", authority).First(&paymentLog).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment log not found"})
		return
	}

	// Retrieve the user ID from the payment log
	var user models.User
	if err := h.db.Where("id = ?", paymentLog.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find user"})
		return
	}

	// Parse amount from JSONB data
	var amount float64
	if err := json.Unmarshal([]byte(paymentLog.Data), &amount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse amount from payment log"})
		return
	}

	// Verify the payment with Zarinpal
	ok, refID, err := h.zarinpalService.VerifyPayment(int(amount), authority)
	if err != nil || !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Payment verification failed"})
		return
	}

	// Update or create user payment record
	var userPayment models.UserPayment
	if err := h.db.Where("user_id = ?", user.ID).First(&userPayment).Error; err != nil {
		// If not found, create a new record
		userPayment = models.UserPayment{
			UserID: user.ID,
			Amount: amount / 100000,
		}
		if err := h.db.Create(&userPayment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user payment record"})
			return
		}
	} else {
		// If found, update the amount
		userPayment.Amount += amount / 100000
		if err := h.db.Save(&userPayment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user payment record"})
			return
		}
	}

	// Create payment log with user ID
	newPaymentLog := models.PaymentLog{
		PaymentID: paymentLog.PaymentID,
		Event:     "zarinpal_payment_completed",
		Data:      fmt.Sprintf(`{"authority": "%s", "ref_id": "%s", "user_id": %d}`, authority, refID, user.ID),
	}

	if err := h.db.Create(&newPaymentLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment log"})
		return
	}

	// Send Telegram notification
	if err := h.telegramService.SendPaymentNotification(
		user.Name,
		user.InstagramID,
		amount,
		"irr",
		time.Now(),
	); err != nil {
		log.Printf("Failed to send Telegram notification: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"ref_id":  refID,
		"amount":  amount,
		"orderId": paymentLog.PaymentID,
	})
}

func (h *PaymentHandler) PreparePayment(c *gin.Context) {
	var req PreparePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if the user already exists
	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new user if not found
			user = models.User{
				Name:        req.Name,
				InstagramID: req.InstagramID,
				Email:       req.Email,
			}
			if err := h.db.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user existence"})
			return
		}
	}

	paymentLog := models.PaymentLog{
		UserID:    user.ID,
		PaymentID: 0,
		Event:     "payment_prepared",
		Data:      fmt.Sprintf(`{"amount": %f, "currency": "%s", "authority_id": "%s"}`, req.Amount, req.Currency, req.AuthorityID),
	}

	if err := h.db.Create(&paymentLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
