package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"vinak/internal/models"
	"vinak/internal/services"
	"vinak/pkg/email"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserHandler struct {
	db           *gorm.DB
	otpService   *services.OTPService
	emailService *email.EmailService
}

func NewUserHandler(db *gorm.DB, otpService *services.OTPService, emailService *email.EmailService) *UserHandler {
	return &UserHandler{
		db:           db,
		otpService:   otpService,
		emailService: emailService,
	}
}

type SendOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type VerifyOTPRequest struct {
	Email       string `json:"email" binding:"required,email"`
	OTP         string `json:"otp" binding:"required"`
	InstagramID string `json:"instagram_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
}

func (h *UserHandler) SendOTP(c *gin.Context) {
	var req SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	otp, err := h.otpService.GenerateOTP(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	fmt.Println(otp)

	err = h.emailService.SendVerificationEmail(req.Email, otp)
	if err != nil {
		fmt.Println(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP sent successfully"})
}

func (h *UserHandler) VerifyOTPAndCreateUser(c *gin.Context) {
	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//valid, err := h.otpService.VerifyOTP(req.Email, req.OTP)
	//if err != nil {
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify OTP"})
	//	return
	//}
	//
	//if !valid {
	//	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
	//	return
	//}

	// Generate API key
	apiKey := make([]byte, 32)
	if _, err := rand.Read(apiKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
		return
	}
	apiKeyStr := base64.StdEncoding.EncodeToString(apiKey)

	// Generate verification token
	verificationToken := make([]byte, 32)
	if _, err := rand.Read(verificationToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
		return
	}
	verificationTokenStr := base64.StdEncoding.EncodeToString(verificationToken)

	user := models.User{
		Email:             req.Email,
		InstagramID:       req.InstagramID,
		Name:              req.Name,
		APIKey:            apiKeyStr,
		VerificationToken: verificationTokenStr,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User created successfully",
		"api_key": apiKeyStr,
	})
}
