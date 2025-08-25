package notifications

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moomoo-trading/api/internal/redis"
)

// NotificationManager manages notifications
type NotificationManager struct {
	streamManager *redis.StreamManager
	providers     map[string]NotificationProvider
}

// NotificationProvider interface for different notification channels
type NotificationProvider interface {
	Send(ctx context.Context, notification *Notification) error
}

// Notification represents a notification
type Notification struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Level     string                 `json:"level"` // "info", "warning", "error"
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(streamManager *redis.StreamManager) *NotificationManager {
	return &NotificationManager{
		streamManager: streamManager,
		providers:     make(map[string]NotificationProvider),
	}
}

// RegisterProvider registers a notification provider
func (nm *NotificationManager) RegisterProvider(name string, provider NotificationProvider) {
	nm.providers[name] = provider
	log.Printf("Registered notification provider: %s", name)
}

// SendNotification sends a notification through all registered providers
func (nm *NotificationManager) SendNotification(ctx context.Context, notification *Notification) error {
	notification.CreatedAt = time.Now()

	// Publish to Redis stream
	event := map[string]interface{}{
		"id":         notification.ID,
		"type":       notification.Type,
		"title":      notification.Title,
		"message":    notification.Message,
		"level":      notification.Level,
		"data":       notification.Data,
		"created_at": notification.CreatedAt.Unix(),
	}

	if err := nm.streamManager.PublishStrategyLog(ctx, event); err != nil {
		log.Printf("Failed to publish notification to stream: %v", err)
	}

	// Send through all providers
	for name, provider := range nm.providers {
		if err := provider.Send(ctx, notification); err != nil {
			log.Printf("Failed to send notification via %s: %v", name, err)
		} else {
			log.Printf("Notification sent via %s: %s", name, notification.Title)
		}
	}

	return nil
}

// SendCircuitBreakerNotification sends a circuit breaker notification
func (nm *NotificationManager) SendCircuitBreakerNotification(ctx context.Context, symbol, reason string) error {
	notification := &Notification{
		ID:      fmt.Sprintf("circuit_%s_%d", symbol, time.Now().Unix()),
		Type:    "circuit_breaker",
		Title:   "Circuit Breaker Triggered",
		Message: fmt.Sprintf("Circuit breaker triggered for %s: %s", symbol, reason),
		Level:   "warning",
		Data: map[string]interface{}{
			"symbol": symbol,
			"reason": reason,
		},
	}

	return nm.SendNotification(ctx, notification)
}

// SendDailySummaryNotification sends a daily summary notification
func (nm *NotificationManager) SendDailySummaryNotification(ctx context.Context, summary map[string]interface{}) error {
	notification := &Notification{
		ID:      fmt.Sprintf("daily_summary_%d", time.Now().Unix()),
		Type:    "daily_summary",
		Title:   "Daily Trading Summary",
		Message: "Daily trading summary is available",
		Level:   "info",
		Data:    summary,
	}

	return nm.SendNotification(ctx, notification)
}

// SendErrorNotification sends an error notification
func (nm *NotificationManager) SendErrorNotification(ctx context.Context, error string, details map[string]interface{}) error {
	notification := &Notification{
		ID:      fmt.Sprintf("error_%d", time.Now().Unix()),
		Type:    "error",
		Title:   "Trading System Error",
		Message: error,
		Level:   "error",
		Data:    details,
	}

	return nm.SendNotification(ctx, notification)
}

// SlackProvider implements NotificationProvider for Slack
type SlackProvider struct {
	webhookURL string
}

// NewSlackProvider creates a new Slack notification provider
func NewSlackProvider(webhookURL string) *SlackProvider {
	return &SlackProvider{
		webhookURL: webhookURL,
	}
}

// Send sends a notification to Slack
func (sp *SlackProvider) Send(ctx context.Context, notification *Notification) error {
	// TODO: Implement actual Slack webhook integration
	log.Printf("Slack notification: %s - %s", notification.Title, notification.Message)
	return nil
}

// EmailProvider implements NotificationProvider for email
type EmailProvider struct {
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	fromEmail    string
	toEmails     []string
}

// NewEmailProvider creates a new email notification provider
func NewEmailProvider(smtpHost string, smtpPort int, smtpUsername, smtpPassword, fromEmail string, toEmails []string) *EmailProvider {
	return &EmailProvider{
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUsername: smtpUsername,
		smtpPassword: smtpPassword,
		fromEmail:    fromEmail,
		toEmails:     toEmails,
	}
}

// Send sends a notification via email
func (ep *EmailProvider) Send(ctx context.Context, notification *Notification) error {
	// TODO: Implement actual email sending
	log.Printf("Email notification: %s - %s", notification.Title, notification.Message)
	return nil
}