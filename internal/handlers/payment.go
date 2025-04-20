package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
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
	Name         string  `json:"name" binding:"required"`
	InstagramID  string  `json:"instagram_id" binding:"required"`
	Email        string  `json:"email" binding:"required,email"`
	Currency     string  `json:"currency" binding:"required,oneof=usd irr"`
	Amount       float64 `json:"amount" binding:"required,gt=0"`
	AuthorityID  string  `json:"authority_id"`
	OrderId      string  `json:"order_id"`
	PurchaseType string  `json:"purchase_type" binding:"required,oneof=physical digital"`
	PersianName  string  `json:"persian_name"`
	PhoneNumber  string  `json:"phone_number"`
	Province     string  `json:"province"`
	City         string  `json:"city"`
	Address      string  `json:"address"`
	PostalCode   string  `json:"postal_code"`
	PlateNumber  string  `json:"plate_number"`
}

func (h *PaymentHandler) HandleNowPaymentsCallback(c *gin.Context) {
	var callback struct {
		PaymentID     int64   `json:"payment_id"`
		PaymentStatus string  `json:"payment_status"`
		PayAddress    string  `json:"pay_address"`
		PriceAmount   float64 `json:"price_amount"`
		PriceCurrency string  `json:"price_currency"`
		PayAmount     float64 `json:"pay_amount"`
		ActuallyPaid  float64 `json:"actually_paid"`
		PayCurrency   string  `json:"pay_currency"`
		OrderID       *string `json:"order_id"`
	}

	if err := c.ShouldBindJSON(&callback); err != nil {
		log.Printf("NowPayments callback error: Failed to bind JSON: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	log.Printf("Received NowPayments callback: %+v", callback)

	// Query the PaymentLog table using order_id from the initial payment preparation
	var paymentLog models.PaymentLog
	if err := h.db.Where("data->>'order_id' = ?", callback.OrderID).First(&paymentLog).Error; err != nil {
		log.Printf("NowPayments callback error: Payment log not found for order_id %v: %v", callback.OrderID, err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Retrieve the user ID from the payment log
	var user models.User
	if err := h.db.Where("id = ?", paymentLog.UserID).First(&user).Error; err != nil {
		log.Printf("NowPayments callback error: Failed to find user: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Verify the payment status
	if callback.PaymentStatus != "finished" {
		log.Printf("NowPayments callback error: Invalid payment status: %s", callback.PaymentStatus)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Calculate original amount (before fees) using the price_amount in USD
	originalAmount := calculateOriginalAmount(callback.PriceAmount, "usd")

	// Calculate storage amount
	storageAmount := originalAmount

	// Update or create user payment record
	var userPayment models.UserPayment
	if err := h.db.Where("user_id = ?", user.ID).First(&userPayment).Error; err != nil {
		// If not found, create a new record
		userPayment = models.UserPayment{
			UserID: user.ID,
			Amount: storageAmount,
		}
		if err := h.db.Create(&userPayment).Error; err != nil {
			log.Printf("NowPayments callback error: Failed to create user payment record: %v", err)
			c.Redirect(http.StatusTemporaryRedirect, "/cancel")
			return
		}
	} else {
		// If found, update the amount
		userPayment.Amount += storageAmount
		if err := h.db.Save(&userPayment).Error; err != nil {
			log.Printf("NowPayments callback error: Failed to update user payment record: %v", err)
			c.Redirect(http.StatusTemporaryRedirect, "/cancel")
			return
		}
	}

	// Create payment log with detailed payment information
	newPaymentLog := models.PaymentLog{
		PaymentID: paymentLog.PaymentID,
		Event:     "nowpayments_payment_completed",
		Data: fmt.Sprintf(`{"payment_id": %d, "status": "%s", "user_id": %d, "price_amount": %f, "price_currency": "%s", "pay_amount": %f, "pay_currency": "%s", "original_amount": %f, "order_id": %s}`,
			callback.PaymentID,
			callback.PaymentStatus,
			user.ID,
			callback.PriceAmount,
			callback.PriceCurrency,
			callback.PayAmount,
			callback.PayCurrency,
			originalAmount,
			func() string {
				if callback.OrderID == nil {
					return "null"
				}
				return fmt.Sprintf(`"%s"`, *callback.OrderID)
			}(),
		),
	}

	if err := h.db.Create(&newPaymentLog).Error; err != nil {
		log.Printf("NowPayments callback error: Failed to create payment log: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// In HandleNowPaymentsCallback
	var purchaseTypeData struct {
		PurchaseType string `json:"purchase_type"`
	}
	if err := json.Unmarshal([]byte(paymentLog.Data), &purchaseTypeData); err != nil {
		log.Printf("NowPayments callback warning: Failed to parse purchase type: %v", err)
	}

	// Send Telegram notification with original amount
	var persianName *string
	var phoneNumber, province, city, address, postalCode, plateNumber *string
	if purchaseTypeData.PurchaseType == "physical" {
		persianName = &user.PersianName
		phoneNumber = user.PhoneNumber
		province = user.Province
		city = user.City
		address = user.Address
		postalCode = user.PostalCode
		plateNumber = user.PlateNumber
	}

	if err := h.telegramService.SendPaymentNotification(
		user.Name,
		user.InstagramID,
		originalAmount,
		callback.PriceCurrency,
		time.Now(),
		purchaseTypeData.PurchaseType,
		persianName,
		phoneNumber,
		province,
		city,
		address,
		postalCode,
		plateNumber,
	); err != nil {
		log.Printf("NowPayments callback warning: Failed to send Telegram notification: %v", err)
	}

	c.Redirect(http.StatusTemporaryRedirect, "/success")
}

func (h *PaymentHandler) GetTopUsers(c *gin.Context) {
	var topUsers []struct {
		Name        string  `json:"name"`
		InstagramID string  `json:"instagram_id"`
		TotalAmount float64 `json:"total_amount"`
	}

	query := h.db.Model(&models.UserPayment{}).
		Select("users.name, users.instagram_id, user_payments.amount as total_amount").
		Joins("JOIN users ON users.id = user_payments.user_id").
		Order("total_amount DESC").
		Limit(10)

	log.Printf("Executing GetTopUsers query...")
	err := query.Scan(&topUsers).Error

	if err != nil {
		log.Printf("GetTopUsers error: Failed to fetch top users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve top users"})
		return
	}

	log.Printf("GetTopUsers found %d users", len(topUsers))
	if len(topUsers) > 0 {
		log.Printf("First user in list: %+v", topUsers[0])
	}

	// Initialize empty array if no users found
	if topUsers == nil {
		log.Printf("GetTopUsers: No users found, initializing empty array")
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

func calculateOriginalAmount(amount float64, currency string) float64 {
	if currency == "irr" {
		return amount / 1.14
	}
	return amount / 1.07
}

func (h *PaymentHandler) HandleZarinpalCallback(c *gin.Context) {
	authority := c.Query("Authority")
	status := c.Query("Status")

	if authority == "" || status != "OK" {
		log.Printf("Zarinpal callback error: Invalid authority or status. Authority: %s, Status: %s", authority, status)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Query the PaymentLog table using JSONB query to find the authority ID
	var paymentLog models.PaymentLog
	if err := h.db.Where("data->>'authority_id' = ?", authority).First(&paymentLog).Error; err != nil {
		log.Printf("Zarinpal callback error: Payment log not found: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Retrieve the user ID from the payment log
	var user models.User
	if err := h.db.Where("id = ?", paymentLog.UserID).First(&user).Error; err != nil {
		log.Printf("Zarinpal callback error: Failed to find user: %v", err)
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
		log.Printf("Zarinpal callback error: Failed to parse payment data: %v. Data: %s", err, paymentLog.Data)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	log.Printf("Zarinpal payment data: %+v", paymentData)
	// Verify the payment with Zarinpal using the original amount
	ok, refID, err := h.zarinpalService.VerifyPayment(int(paymentData.Amount*10), authority)
	if err != nil || !ok {
		log.Printf("Zarinpal callback error: Payment verification failed: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Calculate original amount (before fees)
	originalAmount := calculateOriginalAmount(paymentData.Amount, "irr")

	// After successful verification, convert amount for storage (divide by 100000)
	storageAmount := originalAmount / 83000

	// Update or create user payment record
	var userPayment models.UserPayment
	if err := h.db.Where("user_id = ?", user.ID).First(&userPayment).Error; err != nil {
		// If not found, create a new record
		userPayment = models.UserPayment{
			UserID: user.ID,
			Amount: storageAmount,
		}
		if err := h.db.Create(&userPayment).Error; err != nil {
			log.Printf("Zarinpal callback error: Failed to create user payment record: %v", err)
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

	// Create payment log with user ID and both original and paid amounts
	newPaymentLog := models.PaymentLog{
		PaymentID: paymentLog.PaymentID,
		Event:     "zarinpal_payment_completed",
		Data: fmt.Sprintf(`{"authority": "%s", "ref_id": "%s", "user_id": %d, "paid_amount": %f, "original_amount": %f, "currency": "%s"}`,
			authority, refID, user.ID, paymentData.Amount, originalAmount, paymentData.Currency),
	}

	if err := h.db.Create(&newPaymentLog).Error; err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// In HandleZarinpalCallback
	var purchaseTypeData struct {
		PurchaseType string `json:"purchase_type"`
	}
	if err := json.Unmarshal([]byte(paymentLog.Data), &purchaseTypeData); err != nil {
		log.Printf("Zarinpal callback warning: Failed to parse purchase type: %v", err)
	}

	// Send Telegram notification with original amount
	var persianName *string
	var phoneNumber, province, city, address, postalCode, plateNumber *string
	if purchaseTypeData.PurchaseType == "physical" {
		persianName = &user.PersianName
		phoneNumber = user.PhoneNumber
		province = user.Province
		city = user.City
		address = user.Address
		postalCode = user.PostalCode
		plateNumber = user.PlateNumber
	}

	if err := h.telegramService.SendPaymentNotification(
		user.Name,
		user.InstagramID,
		originalAmount,
		"irr",
		time.Now(),
		purchaseTypeData.PurchaseType,
		persianName,
		phoneNumber,
		province,
		city,
		address,
		postalCode,
		plateNumber,
	); err != nil {
		log.Printf("Failed to send Telegram notification: %v", err)
	}

	// Redirect to success page
	c.Redirect(http.StatusTemporaryRedirect, "/success")
}

func (h *PaymentHandler) HandlePayPalCallback(c *gin.Context) {
	orderCode := c.Query("orderCode")
	if orderCode == "" {
		log.Printf("PayPal callback error: Missing order code")
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Query the PaymentLog table using order_id from the initial payment preparation
	var paymentLog models.PaymentLog
	if err := h.db.Where("data->>'order_id' = ?", orderCode).First(&paymentLog).Error; err != nil {
		log.Printf("PayPal callback error: Payment log not found for order_id %v: %v", orderCode, err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Parse the original payment data
	var paymentData struct {
		Amount       float64 `json:"amount"`
		Currency     string  `json:"currency"`
		OrderID      string  `json:"order_id"`
		PurchaseType string  `json:"purchase_type"`
	}
	if err := json.Unmarshal([]byte(paymentLog.Data), &paymentData); err != nil {
		log.Printf("PayPal callback error: Failed to parse payment data: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Retrieve the user ID from the payment log
	var user models.User
	if err := h.db.Where("id = ?", paymentLog.UserID).First(&user).Error; err != nil {
		log.Printf("PayPal callback error: Failed to find user: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// Calculate original amount (before fees)
	originalAmount := calculateOriginalAmount(paymentData.Amount, strings.ToLower(paymentData.Currency))

	// Calculate storage amount
	storageAmount := originalAmount

	// Update or create user payment record
	var userPayment models.UserPayment
	if err := h.db.Where("user_id = ?", user.ID).First(&userPayment).Error; err != nil {
		// If not found, create a new record
		userPayment = models.UserPayment{
			UserID: user.ID,
			Amount: storageAmount,
		}
		if err := h.db.Create(&userPayment).Error; err != nil {
			log.Printf("PayPal callback error: Failed to create user payment record: %v", err)
			c.Redirect(http.StatusTemporaryRedirect, "/cancel")
			return
		}
	} else {
		// If found, update the amount
		userPayment.Amount += storageAmount
		if err := h.db.Save(&userPayment).Error; err != nil {
			log.Printf("PayPal callback error: Failed to update user payment record: %v", err)
			c.Redirect(http.StatusTemporaryRedirect, "/cancel")
			return
		}
	}

	// Create payment log with detailed payment information
	newPaymentLog := models.PaymentLog{
		PaymentID: paymentLog.PaymentID,
		UserID:    user.ID,
		Event:     "paypal_payment_completed",
		Data: fmt.Sprintf(`{
			"order_id": "%s",
			"status": "completed",
			"amount": %f,
			"currency": "%s",
			"original_amount": %f,
			"purchase_type": "%s"
		}`,
			orderCode,
			paymentData.Amount,
			paymentData.Currency,
			originalAmount,
			paymentData.PurchaseType,
		),
	}

	if err := h.db.Create(&newPaymentLog).Error; err != nil {
		log.Printf("PayPal callback error: Failed to create payment log: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/cancel")
		return
	}

	// In HandlePayPalCallback
	var purchaseTypeData struct {
		PurchaseType string `json:"purchase_type"`
	}
	if err := json.Unmarshal([]byte(paymentLog.Data), &purchaseTypeData); err != nil {
		log.Printf("PayPal callback warning: Failed to parse purchase type: %v", err)
	}

	// Send Telegram notification with original amount and purchase type
	var persianName *string
	var phoneNumber, province, city, address, postalCode, plateNumber *string
	if purchaseTypeData.PurchaseType == "physical" {
		persianName = &user.PersianName
		phoneNumber = user.PhoneNumber
		province = user.Province
		city = user.City
		address = user.Address
		postalCode = user.PostalCode
		plateNumber = user.PlateNumber
	}

	if err := h.telegramService.SendPaymentNotification(
		user.Name,
		user.InstagramID,
		originalAmount,
		strings.ToLower(paymentData.Currency),
		time.Now(),
		purchaseTypeData.PurchaseType,
		persianName,
		phoneNumber,
		province,
		city,
		address,
		postalCode,
		plateNumber,
	); err != nil {
		log.Printf("PayPal callback warning: Failed to send Telegram notification: %v", err)
	}

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
	if req.PurchaseType == "physical" {
		if req.PersianName != "" {
			user.PersianName = req.PersianName
		}
		if user.PhoneNumber == nil || *user.PhoneNumber == "" {
			user.PhoneNumber = &req.PhoneNumber
		}
		if user.Province == nil || *user.Province == "" {
			user.Province = &req.Province
		}
		if user.City == nil || *user.City == "" {
			user.City = &req.City
		}
		if user.Address == nil || *user.Address == "" {
			user.Address = &req.Address
		}
		if user.PostalCode == nil || *user.PostalCode == "" {
			user.PostalCode = &req.PostalCode
		}
		if user.PlateNumber == nil || *user.PlateNumber == "" {
			user.PlateNumber = &req.PlateNumber
		}
		if err := h.db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user"})
			return
		}
	}

	var paymentLog models.PaymentLog
	if req.AuthorityID == "" {
		paymentLog = models.PaymentLog{
			UserID: user.ID,
			Event:  "payment_prepared",
			Data:   fmt.Sprintf(`{"amount": %f, "currency": "%s", "order_id": "%s", "purchase_type": "%s"}`, req.Amount, req.Currency, req.OrderId, req.PurchaseType),
		}
	} else {
		paymentLog = models.PaymentLog{
			UserID:    user.ID,
			PaymentID: 0,
			Event:     "payment_prepared",
			Data:      fmt.Sprintf(`{"amount": %f, "currency": "%s", "authority_id": "%s", "purchase_type": "%s"}`, req.Amount, req.Currency, req.AuthorityID, req.PurchaseType),
		}
	}

	if err := h.db.Create(&paymentLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
