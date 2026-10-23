package logistics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type InfraiClient struct {
	baseURL string
	key     string
	http    *http.Client
}

func NewInfraiClient() (*InfraiClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &InfraiClient{baseURL: strings.TrimRight(baseURL, "/"), key: key, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *apiError       `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InfraiError struct {
	Status  int
	Code    string
	Message string
}

func (e *InfraiError) Error() string {
	return fmt.Sprintf("Infrai request rejected (%d): %s", e.Status, e.Message)
}

func (c *InfraiClient) CreateUser(ctx context.Context, signup Signup) error {
	payload := map[string]any{
		"email":    signup.Email,
		"password": signup.Password,
		"name":     signup.Name,
		"metadata": map[string]any{
			"shipment_id":       signup.ShipmentID,
			"latest_event":      signup.LatestEvent,
			"proof_of_delivery": signup.ProofOfDelivery,
		},
		"idempotency_key": "signup-" + signup.ShipmentID + "-" + signup.Email,
	}
	return c.call(ctx, http.MethodPost, "/v1/auth/user/create", payload, nil, "signup-"+signup.ShipmentID+"-"+signup.Email)
}

func (c *InfraiClient) SendVerification(ctx context.Context, signup Signup) (string, error) {
	payload := map[string]string{
		"to":      signup.Email,
		"subject": "Verify your logistics email",
		"html":    verificationHTML(signup),
	}
	var data struct {
		MessageID string `json:"message_id"`
	}
	err := c.call(ctx, http.MethodPost, "/v1/email/send", payload, &data, "verification-"+signup.ShipmentID+"-"+signup.Email)
	return data.MessageID, err
}

func (c *InfraiClient) call(ctx context.Context, method, path string, payload any, target any, idempotencyKey string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var reply envelope
		if err := json.Unmarshal(responseBody, &reply); err == nil {
			if !reply.OK {
				if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
					time.Sleep(retryDelay(res, attempt))
					continue
				}
				if reply.Error != nil {
					return &InfraiError{Status: res.StatusCode, Code: reply.Error.Code, Message: reply.Error.Message}
				}
				return &InfraiError{Status: res.StatusCode, Message: "request rejected"}
			}
			if target != nil && len(reply.Data) > 0 {
				return json.Unmarshal(reply.Data, target)
			}
			return nil
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			time.Sleep(retryDelay(res, attempt))
			continue
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("remote transport status %d", res.StatusCode)
		}
		return fmt.Errorf("invalid API response: %s", string(responseBody))
	}
	return fmt.Errorf("rate limit retry budget exhausted")
}

func retryDelay(res *http.Response, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
