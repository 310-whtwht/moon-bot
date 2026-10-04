// Package notify sends messages to humans.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var slackClient = &http.Client{Timeout: 10 * time.Second}

// Slack posts text to an incoming webhook.
func Slack(ctx context.Context, webhookURL, text string) error {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := slackClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("slack: http %d: %s", resp.StatusCode, detail)
	}
	return nil
}
