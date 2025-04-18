package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"vinak/internal/models"
	"vinak/pkg/constants"
	"vinak/pkg/errors"
	"vinak/pkg/payment"
	"vinak/pkg/telegram"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PaymentHandler struct {
	db                 *gorm.DB
	nowpaymentsService *payment.NowPaymentsService
	paypalService      *payment.PayPalService
	zarinpalService    *payment.ZarinpalService
	telegramService    *telegram.TelegramService
}

func NewPaymentHandler(db *gorm.DB, nowpaymentsService *payment.NowPaymentsService, paypalService *payment.PayPalService, zarinpalService *payment.ZarinpalService, telegramService *telegram.TelegramService) *PaymentHandler {
	return &PaymentHandler{
		db:                 db,
		nowpaymentsService: nowpaymentsService,
		paypalService:      paypalService,
		zarinpalService:    zarinpalService,
		telegramService:    telegramService,
	}
}

type CreatePaymentRequest struct {
	Amount   float64 `json:"amount" binding:"required,gt=0"`
	Currency string  `json:"currency" binding:"required,oneof=usd irr btc"`
	Gateway  string  `json:"gateway" binding:"required,oneof=zarinpal paypal nowpayments"`
}

type PreparePaymentRequest struct {
	Name        string  `json:"name" binding:"required"`
	InstagramID string  `json:"instagram_id" binding:"required"`
	Email       string  `json:"email" binding:"required,email"`
	Currency    string  `json:"currency" binding:"required,oneof=usd irr btc"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	AuthorityID string  `json:"authority_id" binding:"required"`
}

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	apiKey := c.GetHeader(constants.HeaderAPIKey)
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, errors.NewAPIError(http.StatusUnauthorized, "API key is required"))
		return
	}

	var user models.User
	if err := h.db.Where("api_key = ?", apiKey).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, errors.NewAPIError(http.StatusUnauthorized, "Invalid API key"))
		return
	}

	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.NewAPIError(http.StatusBadRequest, err.Error()))
		return
	}

	// Create payment record
	payment := models.Payment{
		UserID:   user.ID,
		Amount:   req.Amount,
		Status:   constants.PaymentStatusPending,
		Currency: req.Currency,
	}

	if err := h.db.Create(&payment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, "Failed to create payment record"))
		return
	}

	fmt.Println("1")

	switch req.Gateway {
	case constants.PaymentGatewayZarinpal:
		if req.Currency != constants.CurrencyIRR {
			c.JSON(http.StatusBadRequest, errors.NewAPIError(http.StatusBadRequest, constants.ErrZarinpalOnlyIRR))
			return
		}

		// Convert amount to Tomans for Zarinpal
		amountInTomans := int(req.Amount)

		// Create Zarinpal payment
		paymentURL, authority, err := h.zarinpalService.CreatePayment(
			amountInTomans,
			"https://ak47album.com/api/payments/zarinpal/callback",
			"Payment for service",
			user.Email,
			"09352439835",
		)
		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusInternalServerError, errors.NewPaymentGatewayError(constants.PaymentGatewayZarinpal, err))
			return
		}

		// Update payment with Zarinpal authority
		payment.ZarinpalAuthority = &authority
		if err := h.db.Save(&payment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, "Failed to update payment record"))
			return
		}

		// Create payment log
		paymentLog := models.PaymentLog{
			PaymentID: payment.ID,
			Event:     constants.PaymentEventZarinpalCreated,
			Data:      `{"amount": ` + strconv.Itoa(amountInTomans) + `, "currency": "` + constants.CurrencyIRR + `"}`,
		}

		if err := h.db.Create(&paymentLog).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, "Failed to create payment log"))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"payment_url": paymentURL,
			"payment_id":  payment.ID,
			"gateway":     constants.PaymentGatewayZarinpal,
		})

	case constants.PaymentGatewayPayPal:
		if req.Currency != constants.CurrencyUSD {
			c.JSON(http.StatusBadRequest, errors.NewAPIError(http.StatusBadRequest, constants.ErrPayPalOnlyUSD))
			return
		}

		// Create PayPal order
		order, err := h.paypalService.CreateOrder(
			req.Amount,
			req.Currency,
			"Payment for service",
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewPaymentGatewayError(constants.PaymentGatewayPayPal, err))
			return
		}

		// Update payment with PayPal order ID
		payment.PayPalOrderID = &order.ID
		if err := h.db.Save(&payment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, "Failed to update payment record"))
			return
		}

		// Create payment log
		paymentLog := models.PaymentLog{
			PaymentID: payment.ID,
			Event:     constants.PaymentEventPayPalOrderCreated,
			Data:      `{"order_id": "` + order.ID + `", "amount": ` + strconv.FormatFloat(req.Amount, 'f', 2, 64) + `, "currency": "` + constants.CurrencyUSD + `"}`,
		}

		if err := h.db.Create(&paymentLog).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, "Failed to create payment log"))
			return
		}

		// Find the approval URL
		var approvalURL string
		for _, link := range order.Links {
			if link.Rel == "approve" {
				approvalURL = link.Href
				break
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"payment_url": approvalURL,
			"payment_id":  payment.ID,
			"gateway":     constants.PaymentGatewayPayPal,
		})

	case constants.PaymentGatewayNowPayments:
		// Create NowPayments payment
		nowpaymentsPayment, err := h.nowpaymentsService.CreatePayment(
			req.Amount,
			req.Currency,
			strconv.Itoa(int(payment.ID)),
			"Payment for service",
			"http://ak47album.com/api/payments/nowpayments/callback",
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewPaymentGatewayError(constants.PaymentGatewayNowPayments, err))
			return
		}

		// Update payment with NowPayments payment ID
		payment.NowPaymentsPaymentID = &nowpaymentsPayment.PaymentID
		if err := h.db.Save(&payment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, "Failed to update payment record"))
			return
		}

		// Create payment log
		paymentLog := models.PaymentLog{
			PaymentID: payment.ID,
			Event:     constants.PaymentEventNowPaymentsCreated,
			Data:      fmt.Sprintf(`{"payment_id": "%s", "amount": %f, "currency": "%s"}`, nowpaymentsPayment.PaymentID, req.Amount, req.Currency),
		}

		if err := h.db.Create(&paymentLog).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, "Failed to create payment log"))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"payment_id":        nowpaymentsPayment.PaymentID,
			"payment_status":    nowpaymentsPayment.PaymentStatus,
			"pay_address":       nowpaymentsPayment.PayAddress,
			"price_amount":      nowpaymentsPayment.PriceAmount,
			"price_currency":    nowpaymentsPayment.PriceCurrency,
			"pay_amount":        nowpaymentsPayment.PayAmount,
			"pay_currency":      nowpaymentsPayment.PayCurrency,
			"order_id":          nowpaymentsPayment.OrderID,
			"order_description": nowpaymentsPayment.OrderDescription,
			"created_at":        nowpaymentsPayment.CreatedAt,
			"gateway":           constants.PaymentGatewayNowPayments,
		})

	default:
		c.JSON(http.StatusBadRequest, errors.NewAPIError(http.StatusBadRequest, constants.ErrInvalidPaymentGateway))
	}
}

func (h *PaymentHandler) HandlePayPalCallback(c *gin.Context) {
	orderID := c.Query("token")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	// Get payment and user details
	var payment models.Payment
	var user models.User
	if err := h.db.Where("paypal_order_id = ?", orderID).First(&payment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
		return
	}
	if err := h.db.Where("id = ?", payment.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find user"})
		return
	}

	// Capture PayPal order
	if err := h.paypalService.CaptureOrder(orderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to capture PayPal order"})
		return
	}

	// Update payment status
	payment.Status = "completed"
	if err := h.db.Save(&payment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment status"})
		return
	}

	// Create payment log
	paymentLog := models.PaymentLog{
		PaymentID: payment.ID,
		Event:     "paypal_payment_captured",
		Data:      `{"order_id": "` + orderID + `"}`,
	}

	if err := h.db.Create(&paymentLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment log"})
		return
	}

	// Send Telegram notification
	if err := h.telegramService.SendPaymentNotification(
		user.Name,
		user.InstagramID,
		payment.Amount,
		payment.Currency,
		time.Now(),
	); err != nil {
		log.Printf("Failed to send Telegram notification: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
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

	// Get payment and user details
	var payment models.Payment
	var user models.User
	if err := h.db.Where("nowpayments_payment_id = ?", callback.PaymentID).First(&payment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
		return
	}
	if err := h.db.Where("id = ?", payment.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find user"})
		return
	}

	// Update payment status
	payment.Status = "completed"
	if err := h.db.Save(&payment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment status"})
		return
	}

	// Create payment log
	paymentLog := models.PaymentLog{
		PaymentID: payment.ID,
		Event:     "nowpayments_payment_completed",
		Data:      `{"payment_id": "` + callback.PaymentID + `", "status": "` + callback.PaymentStatus + `"}`,
	}

	if err := h.db.Create(&paymentLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment log"})
		return
	}

	// Send Telegram notification
	if err := h.telegramService.SendPaymentNotification(
		user.Name,
		user.InstagramID,
		payment.Amount,
		payment.Currency,
		time.Now(),
	); err != nil {
		log.Printf("Failed to send Telegram notification: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *PaymentHandler) GetTopUsers(c *gin.Context) {
	var usdTopUsers []struct {
		Name        string  `json:"name"`
		InstagramID string  `json:"instagram_id"`
		TotalAmount float64 `json:"total_amount"`
		Currency    string  `json:"currency"`
	}

	err := h.db.Model(&models.Payment{}).
		Select("users.name, users.instagram_id, SUM(payments.amount) as total_amount").
		Joins("JOIN users ON users.id = payments.user_id").
		Where("payments.status = ?", constants.PaymentStatusCompleted).
		Where("payments.currency = ?", constants.CurrencyUSD).
		Group("users.name, users.instagram_id").
		Order("total_amount DESC").
		Limit(10).
		Scan(&usdTopUsers).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, constants.ErrFailedToGetTopUsers))
		return
	}

	// Add currency to USD supporters
	for i := range usdTopUsers {
		usdTopUsers[i].Currency = constants.CurrencyUSD
	}

	// Get top IRR supporters
	var irrTopUsers []struct {
		Name        string  `json:"name"`
		InstagramID string  `json:"instagram_id"`
		TotalAmount float64 `json:"total_amount"`
		Currency    string  `json:"currency"`
	}

	err = h.db.Model(&models.Payment{}).
		Select("users.name, users.instagram_id, SUM(payments.amount) as total_amount").
		Joins("JOIN users ON users.id = payments.user_id").
		Where("payments.status = ?", constants.PaymentStatusCompleted).
		Where("payments.currency = ?", constants.CurrencyIRR).
		Group("users.name, users.instagram_id").
		Order("total_amount DESC").
		Limit(10).
		Scan(&irrTopUsers).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewAPIError(http.StatusInternalServerError, constants.ErrFailedToGetTopUsers))
		return
	}

	// Add currency to IRR supporters
	for i := range irrTopUsers {
		irrTopUsers[i].Currency = constants.CurrencyIRR
	}

	// Format response
	supporters := make([]struct {
		Name      string  `json:"name"`
		Instagram string  `json:"instagram"`
		Amount    float64 `json:"amount"`
		Currency  string  `json:"currency"`
	}, len(usdTopUsers)+len(irrTopUsers))

	// Add USD supporters
	for i, user := range usdTopUsers {
		supporters[i] = struct {
			Name      string  `json:"name"`
			Instagram string  `json:"instagram"`
			Amount    float64 `json:"amount"`
			Currency  string  `json:"currency"`
		}{
			Name:      user.Name,
			Instagram: user.InstagramID,
			Amount:    user.TotalAmount,
			Currency:  user.Currency,
		}
	}

	// Add IRR supporters
	for i, user := range irrTopUsers {
		supporters[len(usdTopUsers)+i] = struct {
			Name      string  `json:"name"`
			Instagram string  `json:"instagram"`
			Amount    float64 `json:"amount"`
			Currency  string  `json:"currency"`
		}{
			Name:      user.Name,
			Instagram: user.InstagramID,
			Amount:    user.TotalAmount,
			Currency:  user.Currency,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"supporters": supporters,
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
			Amount: amount,
		}
		if err := h.db.Create(&userPayment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user payment record"})
			return
		}
	} else {
		// If found, update the amount
		userPayment.Amount += amount
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

	user := models.User{
		Name:        req.Name,
		InstagramID: req.InstagramID,
		Email:       req.Email,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
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
