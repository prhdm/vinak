package models

import (
	"time"
)

type User struct {
	ID                uint    `gorm:"primary_key;auto_increment"`
	Email             string  `gorm:"unique;not null"`
	InstagramID       string  `gorm:"unique;not null"`
	Name              string  `gorm:"not null"`
	APIKey            string  `gorm:"unique;not null"`
	Password          string  `gorm:"not null"`
	EmailVerified     bool    `gorm:"default:false"`
	VerificationToken string  `gorm:"unique"`
	StripeCustomerID  *string `gorm:"default:null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Payment struct {
	ID                   uint    `gorm:"primary_key;auto_increment"`
	UserID               uint    `gorm:"not null"`
	Amount               float64 `gorm:"not null"`
	Status               string  `gorm:"not null"`
	Currency             string  `gorm:"not null"`
	PayPalOrderID        string  `gorm:"unique"`
	NowPaymentsPaymentID string  `gorm:"unique"`
	ZarinpalAuthority    string  `gorm:"unique"`
	ZarinpalRefID        string  `gorm:"unique"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PaymentLog struct {
	ID        uint   `gorm:"primary_key;auto_increment"`
	PaymentID uint   `gorm:"not null"`
	Event     string `gorm:"not null"`
	Data      string `gorm:"type:jsonb"`
	CreatedAt time.Time
}

type TopUserResponse struct {
	UserID       uint    `json:"user_id"`
	Name         string  `json:"name"`
	InstagramID  string  `json:"instagram_id"`
	TotalAmount  float64 `json:"total_amount"`
	Currency     string  `json:"currency"`
	PaymentCount int     `json:"payment_count"`
}
