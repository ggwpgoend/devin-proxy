package keypool

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ggwpgoend/devin-proxy/internal/crypto"
	"github.com/ggwpgoend/devin-proxy/internal/store"
)

func setupPool(t *testing.T) *Pool {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	db, err := store.Open(ctx, filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	cipher, err := crypto.LoadOrCreateCipher(filepath.Join(dir, ".master_key"))
	if err != nil {
		t.Fatal(err)
	}

	return New(db, cipher, 1*time.Minute)
}

func TestAddAndPick(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	id, err := pool.AddKey(ctx, "test-key-123", "test-label", "trial")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	keyID, apiKey, err := pool.Pick(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if keyID != id {
		t.Fatalf("expected keyID=%s, got %s", id, keyID)
	}
	if apiKey != "test-key-123" {
		t.Fatalf("expected apiKey=test-key-123, got %s", apiKey)
	}
}

func TestPickNoKeys(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	_, _, err := pool.Pick(ctx)
	if err != ErrNoActiveKey {
		t.Fatalf("expected ErrNoActiveKey, got %v", err)
	}
}

func TestBulkAdd(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	keys := []string{"key1", "key2", "", "key3", "  "}
	added, err := pool.BulkAdd(ctx, keys, "free")
	if err != nil {
		t.Fatal(err)
	}
	if added != 3 {
		t.Fatalf("expected 3 added, got %d", added)
	}

	all, err := pool.ListKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(all))
	}
}

func TestDisableEnable(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	id, _ := pool.AddKey(ctx, "key-1", "", "trial")

	if err := pool.DisableKey(ctx, id); err != nil {
		t.Fatal(err)
	}
	_, _, err := pool.Pick(ctx)
	if err != ErrNoActiveKey {
		t.Fatal("expected no active key after disable")
	}

	if err := pool.EnableKey(ctx, id); err != nil {
		t.Fatal(err)
	}
	_, apiKey, err := pool.Pick(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if apiKey != "key-1" {
		t.Fatalf("expected key-1, got %s", apiKey)
	}
}

func TestRoundRobin(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	pool.AddKey(ctx, "key-a", "a", "free")
	pool.AddKey(ctx, "key-b", "b", "free")

	_, first, _ := pool.Pick(ctx)
	_, second, _ := pool.Pick(ctx)

	if first == second {
		t.Fatal("expected round-robin to pick different keys")
	}
}

func TestRecordFailureSetsRevoked(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	id, _ := pool.AddKey(ctx, "bad-key", "", "trial")
	pool.RecordFailure(ctx, id, "unauthorized", 401)

	keys, _ := pool.ListKeys(ctx)
	for _, k := range keys {
		if k.ID == id && k.State != "revoked" {
			t.Fatalf("expected revoked, got %s", k.State)
		}
	}
}

func TestRecordFailureSetsCooldown(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	id, _ := pool.AddKey(ctx, "quota-key", "", "trial")
	pool.RecordFailure(ctx, id, "quota exhausted", 402)

	keys, _ := pool.ListKeys(ctx)
	for _, k := range keys {
		if k.ID == id && k.State != "cooldown" {
			t.Fatalf("expected cooldown, got %s", k.State)
		}
	}
}

func TestReactivateExpiredCooldowns(t *testing.T) {
	pool := setupPool(t)
	pool.cooldown = 1 * time.Millisecond
	ctx := context.Background()

	id, _ := pool.AddKey(ctx, "cd-key", "", "trial")
	pool.RecordFailure(ctx, id, "quota", 402)

	time.Sleep(5 * time.Millisecond)
	n, err := pool.ReactivateExpiredCooldowns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 reactivated, got %d", n)
	}
}

func TestStats(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	pool.AddKey(ctx, "k1", "", "trial")
	pool.AddKey(ctx, "k2", "", "free")

	stats, err := pool.GetStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalKeys != 2 {
		t.Fatalf("expected 2 total, got %d", stats.TotalKeys)
	}
	if stats.ActiveKeys != 2 {
		t.Fatalf("expected 2 active, got %d", stats.ActiveKeys)
	}
}

func TestRemoveKey(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	id, _ := pool.AddKey(ctx, "rm-key", "", "trial")
	if err := pool.RemoveKey(ctx, id); err != nil {
		t.Fatal(err)
	}

	err := pool.RemoveKey(ctx, id)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMaskKey(t *testing.T) {
	if m := maskKey("short"); m != "****" {
		t.Fatalf("expected ****, got %s", m)
	}
	if m := maskKey("github_pat_abcdef1234"); m != "gith...1234" {
		t.Fatalf("expected gith...1234, got %s", m)
	}
}

func TestCryptoRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := crypto.LoadOrCreateCipher(filepath.Join(dir, ".key"))
	if err != nil {
		t.Fatal(err)
	}

	enc, err := c.EncryptString("hello world")
	if err != nil {
		t.Fatal(err)
	}

	dec, err := c.DecryptString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", dec)
	}
}

func TestCryptoLoadExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".key")

	c1, _ := crypto.LoadOrCreateCipher(path)
	enc, _ := c1.EncryptString("test")

	c2, _ := crypto.LoadOrCreateCipher(path)
	dec, err := c2.DecryptString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "test" {
		t.Fatalf("expected 'test', got '%s'", dec)
	}
}

func TestCryptoBadKeyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".key")
	os.WriteFile(path, []byte("short"), 0o600)

	_, err := crypto.LoadOrCreateCipher(path)
	if err == nil {
		t.Fatal("expected error for bad key file")
	}
}
