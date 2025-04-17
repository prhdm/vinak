package telegram

import (
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramService struct {
	bot     *tgbotapi.BotAPI
	chatID  int64
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

func (s *TelegramService) SendPaymentNotification(name, instagramID string, amount float64, currency string, paymentTime time.Time) error {
	message := fmt.Sprintf(
		"💰 New Payment Received!\n\n"+
		"👤 Name: %s\n"+
		"📱 Instagram ID: %s\n"+
		"💵 Amount: %.2f %s\n"+
		"⏰ Time: %s",
		name,
		instagramID,
		amount,
		currency,
		paymentTime.Format("2006-01-02 15:04:05"),
	)

	msg := tgbotapi.NewMessage(s.chatID, message)
	_, err := s.bot.Send(msg)
	return err
} 