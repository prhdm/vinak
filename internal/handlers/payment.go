package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
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
	Currency    string  `json:"currency" binding:"required,oneof=crypto irr"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	AuthorityID string  `json:"authority_id" binding:"required"`
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func generateAPIKey() string {
	// Generate a random string for API key
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
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
			Amount: callback.PayAmount,
		}
		if err := h.db.Create(&userPayment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user payment record"})
			return
		}
	} else {
		// If found, update the amount
		userPayment.Amount += callback.PayAmount
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
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Query the PaymentLog table using JSONB query to find the authority ID
	var paymentLog models.PaymentLog
	if err := h.db.Where("data->>'authority_id' = ?", authority).First(&paymentLog).Error; err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Retrieve the user ID from the payment log
	var user models.User
	if err := h.db.Where("id = ?", paymentLog.UserID).First(&user).Error; err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Parse amount from JSONB data
	var paymentData struct {
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		AuthorityID string  `json:"authority_id"`
	}
	if err := json.Unmarshal([]byte(paymentLog.Data), &paymentData); err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	fmt.Println("paymentData", paymentData)
	// Verify the payment with Zarinpal using the original amount
	ok, refID, err := h.zarinpalService.VerifyPayment(int(paymentData.Amount), authority)
	if err != nil || !ok {
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// After successful verification, convert amount for storage (divide by 100000)
	storageAmount := paymentData.Amount / 100000

	// Update or create user payment record
	var userPayment models.UserPayment
	if err := h.db.Where("user_id = ?", user.ID).First(&userPayment).Error; err != nil {
		// If not found, create a new record
		userPayment = models.UserPayment{
			UserID: user.ID,
			Amount: storageAmount,
		}
		if err := h.db.Create(&userPayment).Error; err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/cancel")
			return
		}
	} else {
		// If found, update the amount
		userPayment.Amount += storageAmount
		if err := h.db.Save(&userPayment).Error; err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/cancel")
			return
		}
	}

	// Create payment log with user ID
	newPaymentLog := models.PaymentLog{
		PaymentID: paymentLog.PaymentID,
		Event:     "zarinpal_payment_completed",
		Data:      fmt.Sprintf(`{"authority": "%s", "ref_id": "%s", "user_id": %d, "amount": %f}`, authority, refID, user.ID, paymentData.Amount),
	}

	if err := h.db.Create(&newPaymentLog).Error; err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Send Telegram notification with original amount
	if err := h.telegramService.SendPaymentNotification(
		user.Name,
		user.InstagramID,
		paymentData.Amount,
		"irr",
		time.Now(),
	); err != nil {
		log.Printf("Failed to send Telegram notification: %v", err)
	}

	// Redirect to success page
	c.Redirect(http.StatusTemporaryRedirect, "/success")
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

	// Log payment
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
