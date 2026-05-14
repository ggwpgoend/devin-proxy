package keypool

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ggwpgoend/devin-proxy/internal/crypto"
	"github.com/ggwpgoend/devin-proxy/internal/store"
)

var (
	ErrNoActiveKey = errors.New("keypool: no active key available")
	ErrNotFound    = errors.New("keypool: key not found")
)

const DefaultCooldown = 10 * time.Minute

type Key struct {
	ID            string
	Label         string
	State         string
	PlanType      string
	CooldownUntil *time.Time
	LastUsedAt    *time.Time
	LastError     string
	RequestCount  int64
	SuccessCount  int64
	FailCount     int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Pool struct {
	db       *store.DB
	cipher   *crypto.Cipher
	cooldown time.Duration
	mu       sync.Mutex
}

func New(db *store.DB, cipher *crypto.Cipher, cooldown time.Duration) *Pool {
	if cooldown <= 0 {
		cooldown = DefaultCooldown
	}
	return &Pool{db: db, cipher: cipher, cooldown: cooldown}
}

func (p *Pool) AddKey(ctx context.Context, apiKey, label, planType string) (string, error) {
	enc, err := p.cipher.EncryptString(apiKey)
	if err != nil {
		return "", fmt.Errorf("keypool: encrypt: %w", err)
	}
	id := uuid.New().String()
	if label == "" {
		label = maskKey(apiKey)
	}
	if planType == "" {
		planType = "unknown"
	}
	now := time.Now().UTC()
	_, err = p.db.ExecContext(ctx, `INSERT INTO keys (id, label, api_key_enc, plan_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, label, enc, planType, now, now)
	if err != nil {
		return "", fmt.Errorf("keypool: insert: %w", err)
	}
	return id, nil
}

func (p *Pool) BulkAdd(ctx context.Context, keys []string, planType string) (int, error) {
	added := 0
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, err := p.AddKey(ctx, k, "", planType); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}

func (p *Pool) RemoveKey(ctx context.Context, id string) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("keypool: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Pool) DisableKey(ctx context.Context, id string) error {
	return p.setState(ctx, id, "disabled")
}

func (p *Pool) EnableKey(ctx context.Context, id string) error {
	return p.setState(ctx, id, "active")
}

func (p *Pool) Pick(ctx context.Context) (string, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now().UTC()
	row := p.db.QueryRowContext(ctx, `SELECT id, api_key_enc FROM keys
		WHERE state = 'active'
		AND (cooldown_until IS NULL OR cooldown_until <= ?)
		ORDER BY
			CASE plan_type
				WHEN 'trial' THEN 0
				WHEN 'free'  THEN 1
				WHEN 'paid'  THEN 2
				ELSE 3
			END ASC,
			(last_used_at IS NULL) DESC,
			last_used_at ASC,
			created_at ASC
		LIMIT 1`, now)

	var id, enc string
	if err := row.Scan(&id, &enc); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrNoActiveKey
		}
		return "", "", fmt.Errorf("keypool: pick scan: %w", err)
	}

	apiKey, err := p.cipher.DecryptString(enc)
	if err != nil {
		return "", "", fmt.Errorf("keypool: decrypt: %w", err)
	}

	_, _ = p.db.ExecContext(ctx, `UPDATE keys SET last_used_at = ?, updated_at = ?, request_count = request_count + 1 WHERE id = ?`, now, now, id)
	return id, apiKey, nil
}

func (p *Pool) RecordSuccess(ctx context.Context, id string) {
	now := time.Now().UTC()
	_, _ = p.db.ExecContext(ctx, `UPDATE keys SET success_count = success_count + 1, updated_at = ? WHERE id = ?`, now, id)
}

func (p *Pool) RecordFailure(ctx context.Context, id, errMsg string, statusCode int) {
	now := time.Now().UTC()
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	_, _ = p.db.ExecContext(ctx, `UPDATE keys SET fail_count = fail_count + 1, last_error = ?, updated_at = ? WHERE id = ?`, errMsg, now, id)

	if statusCode == 401 || statusCode == 403 {
		_ = p.setState(ctx, id, "revoked")
	} else if statusCode == 402 || (statusCode == 429 && looksLikeQuota(errMsg)) {
		p.setCooldown(ctx, id, now)
	}
}

func (p *Pool) LogRequest(ctx context.Context, keyID, method, path string, statusCode int, latencyMs int64, errMsg string) {
	_, _ = p.db.ExecContext(ctx, `INSERT INTO request_log (key_id, method, path, status_code, latency_ms, error_msg) VALUES (?, ?, ?, ?, ?, ?)`,
		keyID, method, path, statusCode, latencyMs, errMsg)
}

func (p *Pool) ListKeys(ctx context.Context) ([]Key, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT id, label, state, plan_type, cooldown_until, last_used_at, last_error, request_count, success_count, fail_count, created_at, updated_at FROM keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("keypool: list: %w", err)
	}
	defer rows.Close()
	var keys []Key
	for rows.Next() {
		var k Key
		if err := rows.Scan(&k.ID, &k.Label, &k.State, &k.PlanType, &k.CooldownUntil, &k.LastUsedAt, &k.LastError, &k.RequestCount, &k.SuccessCount, &k.FailCount, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("keypool: scan: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

type Stats struct {
	TotalKeys    int
	ActiveKeys   int
	CooldownKeys int
	DisabledKeys int
	RevokedKeys  int
	TotalReqs    int64
	SuccessReqs  int64
	FailReqs     int64
}

func (p *Pool) GetStats(ctx context.Context) (Stats, error) {
	var s Stats
	row := p.db.QueryRowContext(ctx, `SELECT
		COUNT(*),
		SUM(CASE WHEN state='active' THEN 1 ELSE 0 END),
		SUM(CASE WHEN state='cooldown' THEN 1 ELSE 0 END),
		SUM(CASE WHEN state='disabled' THEN 1 ELSE 0 END),
		SUM(CASE WHEN state='revoked' THEN 1 ELSE 0 END),
		COALESCE(SUM(request_count),0),
		COALESCE(SUM(success_count),0),
		COALESCE(SUM(fail_count),0)
		FROM keys`)
	if err := row.Scan(&s.TotalKeys, &s.ActiveKeys, &s.CooldownKeys, &s.DisabledKeys, &s.RevokedKeys, &s.TotalReqs, &s.SuccessReqs, &s.FailReqs); err != nil {
		return s, fmt.Errorf("keypool: stats: %w", err)
	}
	return s, nil
}

type RecentRequest struct {
	KeyLabel   string
	Method     string
	Path       string
	StatusCode int
	LatencyMs  int64
	ErrorMsg   string
	CreatedAt  time.Time
}

func (p *Pool) RecentRequests(ctx context.Context, limit int) ([]RecentRequest, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := p.db.QueryContext(ctx, `SELECT k.label, r.method, r.path, r.status_code, r.latency_ms, r.error_msg, r.created_at
		FROM request_log r JOIN keys k ON r.key_id = k.id
		ORDER BY r.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("keypool: recent: %w", err)
	}
	defer rows.Close()
	var reqs []RecentRequest
	for rows.Next() {
		var r RecentRequest
		if err := rows.Scan(&r.KeyLabel, &r.Method, &r.Path, &r.StatusCode, &r.LatencyMs, &r.ErrorMsg, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("keypool: scan recent: %w", err)
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}

func (p *Pool) DecryptKeyByID(ctx context.Context, id string) (string, error) {
	var enc string
	err := p.db.QueryRowContext(ctx, `SELECT api_key_enc FROM keys WHERE id = ?`, id).Scan(&enc)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keypool: get enc: %w", err)
	}
	plain, err := p.cipher.DecryptString(enc)
	if err != nil {
		return "", fmt.Errorf("keypool: decrypt: %w", err)
	}
	return plain, nil
}

func (p *Pool) UpdateKeyState(ctx context.Context, id, state, lastError string) error {
	now := time.Now().UTC()
	_, err := p.db.ExecContext(ctx, `UPDATE keys SET state = ?, last_error = ?, updated_at = ? WHERE id = ?`, state, lastError, now, id)
	return err
}

func (p *Pool) ReactivateExpiredCooldowns(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	res, err := p.db.ExecContext(ctx, `UPDATE keys SET state = 'active', cooldown_until = NULL, updated_at = ? WHERE state = 'cooldown' AND cooldown_until IS NOT NULL AND cooldown_until <= ?`, now, now)
	if err != nil {
		return 0, fmt.Errorf("keypool: reactivate: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (p *Pool) setState(ctx context.Context, id, state string) error {
	now := time.Now().UTC()
	res, err := p.db.ExecContext(ctx, `UPDATE keys SET state = ?, updated_at = ? WHERE id = ?`, state, now, id)
	if err != nil {
		return fmt.Errorf("keypool: set state: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Pool) setCooldown(ctx context.Context, id string, now time.Time) {
	until := now.Add(p.cooldown)
	_, _ = p.db.ExecContext(ctx, `UPDATE keys SET state = 'cooldown', cooldown_until = ?, updated_at = ? WHERE id = ?`, until, now, id)
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func looksLikeQuota(s string) bool {
	low := strings.ToLower(s)
	for _, hint := range []string{"quota", "acus", "limit reached", "limit exceeded", "no acus"} {
		if strings.Contains(low, hint) {
			return true
		}
	}
	return false
}
