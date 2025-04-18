package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ZarinpalService struct {
	MerchantID string
	Sandbox    bool
}

func NewZarinpalService(merchantID string, sandbox bool) *ZarinpalService {
	return &ZarinpalService{
		MerchantID: merchantID,
		Sandbox:    sandbox,
	}
}

func (s *ZarinpalService) getBaseURL() string {
	if s.Sandbox {
		return "https://sandbox.zarinpal.com/pg/v4"
	}
	return "https://payment.zarinpal.com/pg/v4"
}

type PaymentRequest struct {
	MerchantID  string          `json:"merchant_id"`
	Amount      int             `json:"amount"`
	CallbackURL string          `json:"callback_url"`
	Description string          `json:"description"`
	Metadata    PaymentMetadata `json:"metadata"`
}

type PaymentMetadata struct {
	Mobile  string `json:"mobile"`
	Email   string `json:"email"`
	OrderID string `json:"order_id"`
}

type PaymentResponse struct {
	Data struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		Authority string `json:"authority"`
		FeeType   string `json:"fee_type"`
		Fee       int    `json:"fee"`
	} `json:"data"`
	Errors struct {
		Message     string   `json:"message"`
		Code        int      `json:"code"`
		Validations []string `json:"validations"`
	} `json:"errors"`
}

type VerificationRequest struct {
	MerchantID string `json:"merchant_id"`
	Amount     int    `json:"amount"`
	Authority  string `json:"authority"`
}

type VerificationResponse struct {
	Data struct {
		Code     int    `json:"code"`
		Message  string `json:"message"`
		CardHash string `json:"card_hash"`
		CardPan  string `json:"card_pan"`
		RefID    int    `json:"ref_id"`
		FeeType  string `json:"fee_type"`
		Fee      int    `json:"fee"`
	} `json:"data"`
	Errors struct {
		Message     string   `json:"message"`
		Code        int      `json:"code"`
		Validations []string `json:"validations"`
	} `json:"errors"`
}

func (s *ZarinpalService) CreatePayment(amount int, callbackURL, description, email, mobile string) (string, string, error) {
	req := PaymentRequest{
		MerchantID:  s.MerchantID,
		Amount:      amount,
		CallbackURL: callbackURL,
		Description: description,
		Metadata: PaymentMetadata{
			Email: email,
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %v", err)
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", s.getBaseURL()+"/payment/request.json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %v", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Printf("Zarinpal response: %s\n", string(body))

	var paymentResp PaymentResponse
	if err := json.Unmarshal(body, &paymentResp); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal response: %v, body: %s", err, string(body))
	}

	if paymentResp.Errors.Code != 0 {
		return "", "", fmt.Errorf("payment request failed with code: %d, message: %s", paymentResp.Errors.Code, paymentResp.Errors.Message)
	}

	if paymentResp.Data.Code != 100 {
		return "", "", fmt.Errorf("payment request failed with code: %d, message: %s", paymentResp.Data.Code, paymentResp.Data.Message)
	}

	paymentURL := fmt.Sprintf("https://%s/pg/StartPay/%s",
		func() string {
			if s.Sandbox {
				return "sandbox.zarinpal.com"
			}
			return "payment.zarinpal.com"
		}(),
		paymentResp.Data.Authority)

	return paymentURL, paymentResp.Data.Authority, nil
}

func (s *ZarinpalService) VerifyPayment(amount int, authority string) (bool, string, error) {
	req := VerificationRequest{
		MerchantID: s.MerchantID,
		Amount:     amount,
		Authority:  authority,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to marshal request: %v", err)
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", s.getBaseURL()+"/payment/verify.json", bytes.NewBuffer(reqBody))
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %v", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return false, "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Printf("Zarinpal verification response: %s\n", string(body))

	var verifyResp VerificationResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return false, "", fmt.Errorf("failed to unmarshal response: %v, body: %s", err, string(body))
	}

	if verifyResp.Errors.Code != 0 {
		return false, "", fmt.Errorf("payment verification failed with code: %d, message: %s", verifyResp.Errors.Code, verifyResp.Errors.Message)
	}

	if verifyResp.Data.Code != 100 && verifyResp.Data.Code != 101 {
		return false, "", fmt.Errorf("payment verification failed with code: %d, message: %s", verifyResp.Data.Code, verifyResp.Data.Message)
	}

	return true, fmt.Sprintf("%d", verifyResp.Data.RefID), nil
}
