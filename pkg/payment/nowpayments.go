package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type NowPaymentsService struct {
	apiKey string
}

func NewNowPaymentsService(apiKey string) *NowPaymentsService {
	return &NowPaymentsService{
		apiKey: apiKey,
	}
}

type NowPaymentsPaymentRequest struct {
	PriceAmount      float64 `json:"price_amount"`
	PriceCurrency    string  `json:"price_currency"`
	PayCurrency      string  `json:"pay_currency"`
	OrderID          string  `json:"order_id"`
	OrderDescription string  `json:"order_description"`
	IPNCallbackURL   string  `json:"ipn_callback_url"`
	SuccessURL       string  `json:"success_url"`
	CancelURL        string  `json:"cancel_url"`
}

type NowPaymentsPaymentResponse struct {
	PaymentID        string  `json:"payment_id"`
	PaymentStatus    string  `json:"payment_status"`
	PayAddress       string  `json:"pay_address"`
	PriceAmount      float64 `json:"price_amount"`
	PriceCurrency    string  `json:"price_currency"`
	PayAmount        float64 `json:"pay_amount"`
	PayCurrency      string  `json:"pay_currency"`
	OrderID          string  `json:"order_id"`
	OrderDescription string  `json:"order_description"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func (s *NowPaymentsService) CreatePayment(amount float64, currency, orderID, description, callbackURL string) (*NowPaymentsPaymentResponse, error) {
	req := NowPaymentsPaymentRequest{
		PriceAmount:      amount,
		PriceCurrency:    currency,
		PayCurrency:      "btc", // Default to BTC, can be changed based on requirements
		OrderID:          orderID,
		OrderDescription: description,
		IPNCallbackURL:   callbackURL,
		SuccessURL:       os.Getenv("NOWPAYMENTS_SUCCESS_URL"),
		CancelURL:        os.Getenv("NOWPAYMENTS_CANCEL_URL"),
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", "https://api.nowpayments.io/v1/payment", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	request.Header.Set("x-api-key", s.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nowpayments API returned status code: %d", response.StatusCode)
	}

	var paymentResp NowPaymentsPaymentResponse
	if err := json.NewDecoder(response.Body).Decode(&paymentResp); err != nil {
		return nil, err
	}

	return &paymentResp, nil
}

func (s *NowPaymentsService) GetPaymentStatus(paymentID string) (*NowPaymentsPaymentResponse, error) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", fmt.Sprintf("https://api.nowpayments.io/v1/payment/%s", paymentID), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("x-api-key", s.apiKey)

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nowpayments API returned status code: %d", response.StatusCode)
	}

	var paymentResp NowPaymentsPaymentResponse
	if err := json.NewDecoder(response.Body).Decode(&paymentResp); err != nil {
		return nil, err
	}

	return &paymentResp, nil
}
