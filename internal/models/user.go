package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;"`
	Email             string    `gorm:"unique;not null"`
	InstagramID       string    `gorm:"unique;not null"`
	Name              string    `gorm:"not null"`
	APIKey            string    `gorm:"unique;not null"`
	Password          string    `gorm:"not null"`
	EmailVerified     bool      `gorm:"default:false"`
	VerificationToken string    `gorm:"unique"`
	StripeCustomerID  string    `gorm:"unique"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Payment struct {
	ID                   uuid.UUID `gorm:"type:uuid;primary_key;"`
	UserID               uuid.UUID `gorm:"type:uuid;not null"`
	Amount               float64   `gorm:"not null"`
	Status               string    `gorm:"not null"`
	Currency             string    `gorm:"not null"`
	PayPalOrderID        string    `gorm:"unique"`
	NowPaymentsPaymentID string    `gorm:"unique"`
	ZarinpalAuthority    string    `gorm:"unique"`
	ZarinpalRefID        string    `gorm:"unique"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PaymentLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	PaymentID uuid.UUID `gorm:"type:uuid;not null"`
	Event     string    `gorm:"not null"`
	Data      string    `gorm:"type:jsonb"`
	CreatedAt time.Time
}

type TopUserResponse struct {
	UserID       uuid.UUID `json:"user_id"`
	Name         string    `json:"name"`
	InstagramID  string    `json:"instagram_id"`
	TotalAmount  float64   `json:"total_amount"`
	Currency     string    `json:"currency"`
	PaymentCount int       `json:"payment_count"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (p *Payment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (pl *PaymentLog) BeforeCreate(tx *gorm.DB) error {
	if pl.ID == uuid.Nil {
		pl.ID = uuid.New()
	}
	return nil
}
