package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ustasjs/gopher-markt/internal/logger"
	"go.uber.org/zap"
)

const (
	retryAttempts  = 3
	retryBaseDelay = 200 * time.Millisecond
)

type AccrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual"`
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	baseURL    string
	httpClient httpDoer
}

func NewClient(address string) *Client {
	baseURL := address
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: newRetryingDoer(
			&http.Client{Timeout: 5 * time.Second},
			retryAttempts,
			retryBaseDelay,
		),
	}
}

func (c *Client) GetOrder(ctx context.Context, number string) (*AccrualResponse, int, error) {
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, number)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("accrual client: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("accrual client: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	var result AccrualResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("accrual client: decode response: %w", err)
	}

	return &result, resp.StatusCode, nil
}

type retryingDoer struct {
	inner    httpDoer
	attempts int
	delay    time.Duration
}

func newRetryingDoer(inner httpDoer, attempts int, delay time.Duration) *retryingDoer {
	return &retryingDoer{inner: inner, attempts: attempts, delay: delay}
}

func (r *retryingDoer) Do(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for attempt := 1; attempt <= r.attempts; attempt++ {
		resp, err = r.inner.Do(req)
		wait, retry := classify(resp, err, r.delay, attempt)
		if !retry || attempt == r.attempts {
			return resp, err
		}
		if resp != nil {
			resp.Body.Close()
		}
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(wait):
		}
	}
	return resp, err
}

func classify(resp *http.Response, err error, baseDelay time.Duration, attempt int) (time.Duration, bool) {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0, false
		}
		return backoff(baseDelay, attempt), true
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		if d := parseRetryAfter(resp); d > 0 {
			return d, true
		}
		return backoff(baseDelay, attempt), true
	case resp.StatusCode >= 500:
		return backoff(baseDelay, attempt), true
	default:
		return 0, false
	}
}

func backoff(base time.Duration, attempt int) time.Duration {
	return base << (attempt - 1)
}

func parseRetryAfter(resp *http.Response) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(v)
	if err != nil {
		logger.Log.Warn("accrual client: parse Retry-After header", zap.String("value", v), zap.Error(err))
		return 0
	}
	if secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
