package main

import (
	"fmt"
	"log/slog"
)

type NotificationService interface {
	SendNotification(recipient, notificationType, message string) error
}

type NotificationServiceImpl struct {
	rateLimiter RateLimiter
}

func NewNotificationService(rateLimiter RateLimiter) *NotificationServiceImpl {
	return &NotificationServiceImpl{
		rateLimiter: rateLimiter,
	}
}

func (s *NotificationServiceImpl) SendNotification(recipient, notificationType, message string) error {
	allowed, err := s.rateLimiter.Allow(recipient, notificationType)
	if err != nil {
		// even though the in-memory rate limiter doesn't return errors,
		// the interface still has an error return value for implementations that can have unexpected errors
		// so that the caller can decide whether to fail open or closed in such cases.
		// Here we log the error and fail open as notifications should not be blocked due to rate limiter errors
		slog.Error("error checking rate limit", "error", err, "recipient", recipient, "type", notificationType)
	} else if !allowed {
		return fmt.Errorf("rate limit exceeded for %s notifications to %s", notificationType, recipient)
	}

	fmt.Printf("Sending %s notification to %s: %s\n", notificationType, recipient, message)
	return nil
}
