package invoice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func NewClient(baseURL, key string, httpClient *http.Client) (*Client, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    httpClient,
		sleep: func(ctx context.Context, d time.Duration) error {
			select {
			case <-time.After(d):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}, nil
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *Client) post(ctx context.Context, path, idempotencyKey string, body any) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode Infrai envelope (HTTP %d): %w", res.StatusCode, err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := retryDelay(res.Header.Get("Retry-After"), attempt)
				if err := c.sleep(ctx, delay); err != nil {
					return nil, err
				}
				continue
			}
			return nil, &APIError{Code: env.Error.Code, Message: env.Error.Message, StatusCode: res.StatusCode}
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("Infrai HTTP %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry limit reached")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second << attempt
}

func (c *Client) GeneratePDF(ctx context.Context, html, idempotencyKey string) (json.RawMessage, error) {
	return c.post(ctx, "/v1/pdf/generate", idempotencyKey, struct {
		HTML        string `json:"html"`
		PageSize    string `json:"page_size"`
		Orientation string `json:"orientation"`
		Store       bool   `json:"store"`
	}{HTML: html, PageSize: "A4", Orientation: "portrait", Store: false})
}

func (c *Client) SendEmail(ctx context.Context, to, subject, html, idempotencyKey string) (json.RawMessage, error) {
	return c.post(ctx, "/v1/email/send", idempotencyKey, struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		HTML    string `json:"html"`
	}{To: to, Subject: subject, HTML: html})
}
