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
		return "https://sandbox.zarinpal.com/pg/rest/WebGate"
	}
	return "https://www.zarinpal.com/pg/rest/WebGate"
}

type PaymentRequest struct {
	MerchantID  string `json:"MerchantID"`
	Amount      int    `json:"Amount"`
	CallbackURL string `json:"CallbackURL"`
	Description string `json:"Description"`
	Email       string `json:"Email"`
	Mobile      string `json:"Mobile"`
}

type PaymentResponse struct {
	Status    int    `json:"Status"`
	Authority string `json:"Authority"`
}

type VerificationRequest struct {
	MerchantID string `json:"MerchantID"`
	Amount     int    `json:"Amount"`
	Authority  string `json:"Authority"`
}

type VerificationResponse struct {
	Status int    `json:"Status"`
	RefID  string `json:"RefID"`
}

func (s *ZarinpalService) CreatePayment(amount int, callbackURL, description, email, mobile string) (string, string, error) {
	req := PaymentRequest{
		MerchantID:  s.MerchantID,
		Amount:      amount,
		CallbackURL: callbackURL,
		Description: description,
		Email:       email,
		Mobile:      mobile,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := http.Post(s.getBaseURL()+"/PaymentRequest.json", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Log the response for debugging
	fmt.Printf("Zarinpal response: %s\n", string(body))

	var paymentResp PaymentResponse
	if err := json.Unmarshal(body, &paymentResp); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal response: %v, body: %s", err, string(body))
	}

	if paymentResp.Status != 100 {
		return "", "", fmt.Errorf("payment request failed with status: %d", paymentResp.Status)
	}

	paymentURL := fmt.Sprintf("https://%s/pg/StartPay/%s",
		func() string {
			if s.Sandbox {
				return "sandbox.zarinpal.com"
			}
			return "www.zarinpal.com"
		}(),
		paymentResp.Authority)

	return paymentURL, paymentResp.Authority, nil
}

func (s *ZarinpalService) VerifyPayment(amount int, authority string) (bool, string, error) {
	req := VerificationRequest{
		MerchantID: s.MerchantID,
		Amount:     amount,
		Authority:  authority,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return false, "", err
	}

	resp, err := http.Post(s.getBaseURL()+"/PaymentVerification.json", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	var verifyResp VerificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return false, "", err
	}

	if verifyResp.Status != 100 {
		return false, "", fmt.Errorf("payment verification failed with status: %d", verifyResp.Status)
	}

	return true, verifyResp.RefID, nil
}
