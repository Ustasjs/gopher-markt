package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AccrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(address string) *Client {
	baseURL := address
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) GetOrder(ctx context.Context, number string) (response *AccrualResponse, httpStatus int, retryAfter time.Duration, error error) {
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, number)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("accrual client: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("accrual client: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, resp.StatusCode, parseRetryAfter(resp), nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, 0, nil
	}

	var result AccrualResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("accrual client: decode response: %w", err)
	}

	return &result, resp.StatusCode, 0, nil
}

func parseRetryAfter(resp *http.Response) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return time.Minute
}
