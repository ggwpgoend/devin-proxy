package proxy

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ggwpgoend/devin-proxy/internal/keypool"
)

const (
	targetHost = "api.devin.ai"
	targetBase = "https://api.devin.ai"
	maxRetries = 3
)

type Handler struct {
	pool   *keypool.Pool
	client *http.Client
}

func NewHandler(pool *keypool.Pool) *Handler {
	return &Handler{
		pool: pool,
		client: &http.Client{
			Timeout: 120 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadGateway)
		return
	}
	defer r.Body.Close()

	var lastStatus int
	for attempt := range maxRetries {
		keyID, apiKey, pickErr := h.pool.Pick(ctx)
		if pickErr != nil {
			log.Printf("[proxy] no active key available: %v", pickErr)
			http.Error(w, `{"error":"no active API key available — add keys via admin panel at :9091"}`, http.StatusServiceUnavailable)
			return
		}

		targetURL := targetBase + r.URL.RequestURI()
		proxyReq, reqErr := http.NewRequestWithContext(ctx, r.Method, targetURL, strings.NewReader(string(body)))
		if reqErr != nil {
			http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
			return
		}

		copyHeaders(r.Header, proxyReq.Header)
		proxyReq.Header.Set("Authorization", "Bearer "+apiKey)
		proxyReq.Header.Set("Host", targetHost)
		proxyReq.Header.Del("X-Forwarded-For")

		start := time.Now()
		resp, doErr := h.client.Do(proxyReq)
		latencyMs := time.Since(start).Milliseconds()

		if doErr != nil {
			h.pool.LogRequest(ctx, keyID, r.Method, r.URL.Path, 0, latencyMs, doErr.Error())
			h.pool.RecordFailure(ctx, keyID, doErr.Error(), 0)
			log.Printf("[proxy] attempt %d: transport error: %v", attempt+1, doErr)
			if attempt < maxRetries-1 {
				continue
			}
			http.Error(w, "upstream unreachable", http.StatusBadGateway)
			return
		}

		lastStatus = resp.StatusCode
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		h.pool.LogRequest(ctx, keyID, r.Method, r.URL.Path, resp.StatusCode, latencyMs, "")

		if shouldRotate(resp.StatusCode, string(respBody)) {
			h.pool.RecordFailure(ctx, keyID, http.StatusText(resp.StatusCode), resp.StatusCode)
			log.Printf("[proxy] attempt %d: key %s got %d, rotating...", attempt+1, keyID[:8], resp.StatusCode)
			if attempt < maxRetries-1 {
				continue
			}
			copyHeaders(resp.Header, w.Header())
			w.WriteHeader(resp.StatusCode)
			w.Write(respBody)
			return
		}

		h.pool.RecordSuccess(ctx, keyID)
		copyHeaders(resp.Header, w.Header())
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	http.Error(w, "all keys exhausted", lastStatus)
}

func shouldRotate(code int, body string) bool {
	if code == 401 || code == 403 || code == 402 {
		return true
	}
	if code == 429 {
		low := strings.ToLower(body)
		for _, hint := range []string{"quota", "acus", "limit"} {
			if strings.Contains(low, hint) {
				return true
			}
		}
		return true
	}
	return false
}

func copyHeaders(src, dst http.Header) {
	for k, vv := range src {
		k2 := strings.ToLower(k)
		if k2 == "host" || k2 == "authorization" || k2 == "content-length" || k2 == "transfer-encoding" {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func StartReactivator(ctx context.Context, pool *keypool.Pool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := pool.ReactivateExpiredCooldowns(ctx); err == nil && n > 0 {
				log.Printf("[pool] reactivated %d keys from cooldown", n)
			}
		}
	}
}
