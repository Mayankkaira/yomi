// Package instclient is a minimal HTTP client used by the consent service to
// authenticate institution API keys against the institution service. This
// keeps schema ownership isolated: consent never touches the institution
// database directly.
package instclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yomi/yomi/internal/models"
)

// Client talks to the institution service over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
	secret     string
}

// NewClient builds a client for the institution service. `secret` is the
// shared internal-service secret used to authorize internal endpoints.
func NewClient(baseURL, secret string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 8 * time.Second},
		secret:     secret,
	}
}

// Authenticate validates an institution API key and returns the institution.
// It returns ErrInvalidKey when the key is unknown or the institution is
// inactive, and ErrUnreachable when the institution service cannot be reached.
func (c *Client) Authenticate(ctx context.Context, apiKey string) (models.Institution, error) {
	body, err := json.Marshal(map[string]string{"apiKey": apiKey})
	if err != nil {
		return models.Institution{}, fmt.Errorf("marshal authenticate request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/institution/authenticate", bytes.NewReader(body))
	if err != nil {
		return models.Institution{}, fmt.Errorf("build authenticate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Yomi-Internal", c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.Institution{}, ErrUnreachable
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return models.Institution{}, fmt.Errorf("read authenticate response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var envelope struct {
			Data models.Institution `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return models.Institution{}, fmt.Errorf("decode authenticate response: %w", err)
		}
		return envelope.Data, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return models.Institution{}, ErrInvalidKey
	default:
		return models.Institution{}, fmt.Errorf("institution service returned %d", resp.StatusCode)
	}
}

var (
	// ErrInvalidKey is returned when the institution API key is invalid.
	ErrInvalidKey = fmt.Errorf("invalid institution api key")
	// ErrUnreachable is returned when the institution service is unavailable.
	ErrUnreachable = fmt.Errorf("institution service unreachable")
)
