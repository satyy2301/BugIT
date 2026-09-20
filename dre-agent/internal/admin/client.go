package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	base string
	http *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		base: baseURL,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) TriggerSnapshot(ctx context.Context, reason, detail string) error {
	if c.base == "" {
		return fmt.Errorf("collector HTTP base URL not configured")
	}
	body, err := json.Marshal(map[string]string{
		"reason": reason,
		"detail": detail,
	})
	if err != nil {
		return err
	}
	url := c.base + "/v1/trigger"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("trigger snapshot: HTTP %d", resp.StatusCode)
	}
	return nil
}
