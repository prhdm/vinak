package telegram

import (
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramService struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

func NewTelegramService(token string, chatID int64) (*TelegramService, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &TelegramService{
		bot:    bot,
		chatID: chatID,
	}, nil
}

func (s *TelegramService) SendPaymentNotification(name, instagramID string, amount float64, currency string, paymentTime time.Time, purchaseType string, persianName, province, city, address, postalCode, plateNumber *string) error {
	// Start with basic message
	message := fmt.Sprintf(
		"💰 New %s Received!\n\n"+
			"👤 Name: %s\n"+
			"📱 Instagram ID: %s\n"+
			"💵 Amount: %.2f %s\n"+
			"🏷️ Type: %s\n"+
			"⏰ Time: %s",
		func() string {
			if purchaseType == "digital" {
				return "Digital Payment"
			}
			return "Physical Purchase"
		}(),
		name,
		instagramID,
		amount,
		currency,
		purchaseType,
		paymentTime.Format("2006-01-02 15:04:05"),
	)

	// Add physical purchase details if applicable
	if purchaseType == "physical" {
		physicalDetails := "\n\n📦 Physical Purchase Details:"

		if persianName != nil && *persianName != "" {
			physicalDetails += fmt.Sprintf("\n🔤 Persian Name: %s", *persianName)
		}
		if province != nil && *province != "" {
			physicalDetails += fmt.Sprintf("\n📍 Province: %s", *province)
		}
		if city != nil && *city != "" {
			physicalDetails += fmt.Sprintf("\n🌆 City: %s", *city)
		}
		if address != nil && *address != "" {
			physicalDetails += fmt.Sprintf("\n📮 Address: %s", *address)
		}
		if postalCode != nil && *postalCode != "" {
			physicalDetails += fmt.Sprintf("\n📫 Postal Code: %s", *postalCode)
		}
		if plateNumber != nil && *plateNumber != "" {
			physicalDetails += fmt.Sprintf("\n🚗 Plate Number: %s", *plateNumber)
		}

		message += physicalDetails
	}

	msg := tgbotapi.NewMessage(s.chatID, message)
	_, err := s.bot.Send(msg)
	return err
}
