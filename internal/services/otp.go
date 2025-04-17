package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
)

type OTPService struct {
	redis *redis.Client
}

func NewOTPService(redis *redis.Client) *OTPService {
	return &OTPService{
		redis: redis,
	}
}

func (s *OTPService) GenerateOTP(email string) (string, error) {
	rand.Seed(time.Now().UnixNano())
	otp := fmt.Sprintf("%06d", rand.Intn(1000000))

	ctx := context.Background()
	err := s.redis.Set(ctx, "otp:"+email, otp, 5*time.Minute).Err()
	if err != nil {
		return "", err
	}

	return otp, nil
}

func (s *OTPService) VerifyOTP(email, otp string) (bool, error) {
	ctx := context.Background()
	storedOTP, err := s.redis.Get(ctx, "otp:"+email).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}

	return storedOTP == otp, nil
}
