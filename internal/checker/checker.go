package checker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const devinAPIBase = "https://api.devin.ai/v1"

type Status string

const (
	StatusValid          Status = "valid"
	StatusUnauthorized   Status = "unauthorized"
	StatusQuotaExhausted Status = "quota_exhausted"
	StatusRateLimited    Status = "rate_limited"
	StatusNetworkError   Status = "network_error"
	StatusAPIError       Status = "api_error"
)

type Result struct {
	Status     Status `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Error      string `json:"error,omitempty"`
	Latency    int64  `json:"latency_ms"`
}

func (r Result) IsHealthy() bool {
	return r.Status == StatusValid
}

func (r Result) Label() string {
	switch r.Status {
	case StatusValid:
		return "Valid"
	case StatusUnauthorized:
		return "Unauthorized"
	case StatusQuotaExhausted:
		return "Quota exhausted"
	case StatusRateLimited:
		return "Rate limited"
	case StatusNetworkError:
		return "Network error"
	default:
		return "API error"
	}
}

func Check(ctx context.Context, apiKey string) Result {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, devinAPIBase+"/sessions?limit=1", nil)
	if err != nil {
		return Result{Status: StatusNetworkError, Error: err.Error(), Latency: time.Since(start).Milliseconds()}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "devin-proxy/checker")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{Status: StatusNetworkError, Error: err.Error(), Latency: latency}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	return classify(resp.StatusCode, body, latency)
}

func classify(code int, body []byte, latency int64) Result {
	switch {
	case code >= 200 && code < 300:
		return Result{Status: StatusValid, HTTPStatus: code, Latency: latency}
	case code == http.StatusPaymentRequired:
		return Result{Status: StatusQuotaExhausted, HTTPStatus: code, Error: "quota exhausted (HTTP 402)", Latency: latency}
	case code == http.StatusTooManyRequests:
		if looksLikeQuota(body) {
			return Result{Status: StatusQuotaExhausted, HTTPStatus: code, Error: "quota exhausted (HTTP 429)", Latency: latency}
		}
		return Result{Status: StatusRateLimited, HTTPStatus: code, Error: "rate limited (HTTP 429)", Latency: latency}
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return Result{Status: StatusUnauthorized, HTTPStatus: code, Error: fmt.Sprintf("unauthorized (HTTP %d)", code), Latency: latency}
	default:
		return Result{
			Status:     StatusAPIError,
			HTTPStatus: code,
			Error:      fmt.Sprintf("api error %d: %s", code, truncate(string(body), 200)),
			Latency:    latency,
		}
	}
}

func looksLikeQuota(body []byte) bool {
	low := strings.ToLower(string(body))
	for _, hint := range []string{"quota", "acus", "limit reached", "limit exceeded", "no acus"} {
		if strings.Contains(low, hint) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
