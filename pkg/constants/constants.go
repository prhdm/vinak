package constants

const (
	// Payment Statuses
	PaymentStatusPending   = "pending"
	PaymentStatusCompleted = "completed"
	PaymentStatusFailed    = "failed"

	// Payment Gateways
	PaymentGatewayZarinpal    = "zarinpal"
	PaymentGatewayPayPal      = "paypal"
	PaymentGatewayNowPayments = "nowpayments"

	// Currencies
	CurrencyUSD = "usd"
	CurrencyIRR = "irr"
	CurrencyBTC = "btc"

	// Payment Events
	PaymentEventZarinpalCreated      = "zarinpal_payment_created"
	PaymentEventPayPalOrderCreated   = "paypal_order_created"
	PaymentEventNowPaymentsCreated   = "nowpayments_payment_created"
	PaymentEventPayPalCaptured       = "paypal_payment_captured"
	PaymentEventNowPaymentsCompleted = "nowpayments_payment_completed"

	// API Headers
	HeaderAPIKey = "X-API-Key"

	// PayPal Constants
	PayPalIntentCapture = "CAPTURE"
	PayPalModeSandbox   = "sandbox"
	PayPalModeLive      = "live"

	// NowPayments Constants
	NowPaymentsDefaultPayCurrency = "btc"

	// Error Messages
	ErrAPIKeyRequired        = "API key is required"
	ErrInvalidAPIKey         = "Invalid API key"
	ErrInvalidPaymentGateway = "Invalid payment gateway"
	ErrInvalidCurrency       = "Invalid currency. Must be 'usd' or 'irr'"
	ErrPaymentNotFound       = "Payment not found"
	ErrUserNotFound          = "Failed to find user"
	ErrFailedToCreatePayment = "Failed to create payment record"
	ErrFailedToUpdatePayment = "Failed to update payment status"
	ErrFailedToCreateLog     = "Failed to create payment log"
	ErrFailedToGetTopUsers   = "Failed to get top users"

	// Zarinpal Specific
	ErrZarinpalOnlyIRR = "Zarinpal only supports IRR currency"
	ErrPayPalOnlyUSD   = "PayPal only supports USD currency"
)
