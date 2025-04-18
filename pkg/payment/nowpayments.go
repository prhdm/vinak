package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type NowPaymentsErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (s *NowPaymentsService) CreatePayment(priceAmount float64, priceCurrency, orderID, orderDescription, callbackURL string) (*NowPaymentsPaymentResponse, error) {
	req := NowPaymentsPaymentRequest{
		PriceAmount:      priceAmount,
		PriceCurrency:    priceCurrency,
		PayCurrency:      "btc", // Default to BTC
		OrderID:          orderID,
		OrderDescription: orderDescription,
		IPNCallbackURL:   callbackURL,
		SuccessURL:       callbackURL + "?status=success",
		CancelURL:        callbackURL + "?status=cancel",
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", "https://api.nowpayments.io/v1/payment", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-api-key", s.apiKey)

	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Printf("NowPayments response: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		var errorResp NowPaymentsErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error response: %v", err)
		}
		return nil, fmt.Errorf("payment request failed with code %d: %s", errorResp.Code, errorResp.Message)
	}

	var paymentResp NowPaymentsPaymentResponse
	if err := json.Unmarshal(body, &paymentResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &paymentResp, nil
}

func (s *NowPaymentsService) GetPaymentStatus(paymentID string) (*NowPaymentsPaymentResponse, error) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", "https://api.nowpayments.io/v1/payment/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	request.Header.Set("x-api-key", s.apiKey)

	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Printf("NowPayments status response: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		var errorResp NowPaymentsErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error response: %v", err)
		}
		return nil, fmt.Errorf("payment status request failed with code %d: %s", errorResp.Code, errorResp.Message)
	}

	var paymentResp NowPaymentsPaymentResponse
	if err := json.Unmarshal(body, &paymentResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &paymentResp, nil
}
