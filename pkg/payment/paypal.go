package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type PayPalService struct {
	clientID     string
	clientSecret string
	mode         string
	accessToken  string
}

func NewPayPalService(clientID, clientSecret, mode string) (*PayPalService, error) {
	service := &PayPalService{
		clientID:     clientID,
		clientSecret: clientSecret,
		mode:         mode,
	}

	// Get access token
	if err := service.getAccessToken(); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *PayPalService) getAccessToken() error {
	client := &http.Client{}
	req, err := http.NewRequest("POST", s.getBaseURL()+"/v1/oauth2/token", bytes.NewBuffer([]byte("grant_type=client_credentials")))
	if err != nil {
		return err
	}

	req.SetBasicAuth(s.clientID, s.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	s.accessToken = result.AccessToken
	return nil
}

func (s *PayPalService) getBaseURL() string {
	if s.mode == "sandbox" {
		return "https://api.sandbox.paypal.com"
	}
	return "https://api.paypal.com"
}

type PayPalOrderRequest struct {
	Intent             string             `json:"intent"`
	PurchaseUnits      []PurchaseUnit     `json:"purchase_units"`
	ApplicationContext ApplicationContext `json:"application_context"`
}

type PurchaseUnit struct {
	Amount      Amount `json:"amount"`
	Description string `json:"description"`
}

type Amount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

type ApplicationContext struct {
	ReturnURL string `json:"return_url"`
	CancelURL string `json:"cancel_url"`
}

type PayPalOrderResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Links  []Link `json:"links"`
}

type Link struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

func (s *PayPalService) CreateOrder(amount float64, currency, description string) (*PayPalOrderResponse, error) {
	req := PayPalOrderRequest{
		Intent: "CAPTURE",
		PurchaseUnits: []PurchaseUnit{
			{
				Amount: Amount{
					CurrencyCode: currency,
					Value:        fmt.Sprintf("%.2f", amount),
				},
				Description: description,
			},
		},
		ApplicationContext: ApplicationContext{
			ReturnURL: os.Getenv("PAYPAL_RETURN_URL"),
			CancelURL: os.Getenv("PAYPAL_CANCEL_URL"),
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", s.getBaseURL()+"/v2/checkout/orders", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", "Bearer "+s.accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("paypal API returned status code: %d", response.StatusCode)
	}

	var orderResp PayPalOrderResponse
	if err := json.NewDecoder(response.Body).Decode(&orderResp); err != nil {
		return nil, err
	}

	return &orderResp, nil
}

func (s *PayPalService) CaptureOrder(orderID string) error {
	client := &http.Client{}
	request, err := http.NewRequest("POST", s.getBaseURL()+"/v2/checkout/orders/"+orderID+"/capture", nil)
	if err != nil {
		return err
	}

	request.Header.Set("Authorization", "Bearer "+s.accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("paypal API returned status code: %d", response.StatusCode)
	}

	return nil
}
