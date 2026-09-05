package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/pkg/aksk"
	"scm/internal/repo"
)

func newTestApiKeyService(t *testing.T) *ApiKeyService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Single connection so the :memory: schema is shared across the test.
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.ApiKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewApiKeyRepo(repository.NewGormDB(db))
	return NewApiKeyService(repo, "test-secret")
}

func TestApiKeyCreateAndVerify(t *testing.T) {
	svc := newTestApiKeyService(t)
	key, sk, err := svc.CreateKey(1, 1, "agent-1", "po:create,material:manage", nil)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if key.AK == "" || sk == "" {
		t.Fatal("empty ak/sk")
	}
	if key.SKCipher == sk || key.SKCipher == "" {
		t.Fatal("SK must be stored as a ciphertext, never plaintext")
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bodyHash := aksk.SHA256Hex([]byte(`{"x":1}`))

	// Correct signature passes and resolves tenant + permission set.
	sig := aksk.Sign(key.AK, ts, "POST", "/api/v1/po", bodyHash, sk)
	actor, err := svc.Verify(key.AK, ts, sig, "POST", "/api/v1/po", bodyHash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if actor.TenantID != 1 || !actor.KeyAuth {
		t.Fatalf("bad actor: %+v", actor)
	}
	if len(actor.KeyPerms) != 2 || actor.KeyPerms[0] != "po:create" || actor.KeyPerms[1] != "material:manage" {
		t.Fatalf("bad perms: %v", actor.KeyPerms)
	}

	// Tampered signature is rejected.
	if _, err := svc.Verify(key.AK, ts, "deadbeef", "POST", "/api/v1/po", bodyHash); err == nil {
		t.Fatal("expected signature mismatch error")
	}

	// A signature over a different body is rejected.
	otherHash := aksk.SHA256Hex([]byte(`{"x":2}`))
	if _, err := svc.Verify(key.AK, ts, sig, "POST", "/api/v1/po", otherHash); err == nil {
		t.Fatal("expected body mismatch error")
	}

	// Stale timestamp is rejected (anti-replay).
	old := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	oldSig := aksk.Sign(key.AK, old, "POST", "/api/v1/po", bodyHash, sk)
	if _, err := svc.Verify(key.AK, old, oldSig, "POST", "/api/v1/po", bodyHash); err == nil {
		t.Fatal("expected stale timestamp error")
	}
}

func TestApiKeyDisable(t *testing.T) {
	svc := newTestApiKeyService(t)
	key, sk, err := svc.CreateKey(1, 1, "agent-2", "", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Disable(1, 1, key.ID); err != nil {
		t.Fatalf("disable: %v", err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bodyHash := aksk.SHA256Hex(nil)
	sig := aksk.Sign(key.AK, ts, "GET", "/api/v1/materials", bodyHash, sk)
	if _, err := svc.Verify(key.AK, ts, sig, "GET", "/api/v1/materials", bodyHash); err == nil {
		t.Fatal("expected disabled key error")
	}
}

func TestApiKeyEmptyPermsMeansAll(t *testing.T) {
	svc := newTestApiKeyService(t)
	key, sk, err := svc.CreateKey(1, 1, "agent-3", "", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bodyHash := aksk.SHA256Hex(nil)
	sig := aksk.Sign(key.AK, ts, "GET", "/api/v1/materials", bodyHash, sk)
	actor, err := svc.Verify(key.AK, ts, sig, "GET", "/api/v1/materials", bodyHash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(actor.KeyPerms) != 0 {
		t.Fatalf("expected empty perms (all), got %v", actor.KeyPerms)
	}
}

func TestApiKeyExpiry(t *testing.T) {
	svc := newTestApiKeyService(t)
	exp := time.Now().Add(-time.Hour)
	key, sk, err := svc.CreateKey(1, 1, "agent-4", "", &exp)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bodyHash := aksk.SHA256Hex(nil)
	sig := aksk.Sign(key.AK, ts, "GET", "/api/v1/materials", bodyHash, sk)
	if _, err := svc.Verify(key.AK, ts, sig, "GET", "/api/v1/materials", bodyHash); err == nil {
		t.Fatal("expected expired key error")
	}
}
