package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"scm/internal/model"
	"scm/internal/pkg/aksk"
	"scm/internal/pkg/authx"
	"scm/internal/repository"
)

const (
	akPrefix = "ak_"
	skPrefix = "sk_"
)

// ApiKeyService owns AK/SK issuance, lifecycle and signature verification.
// The SK is returned in plaintext only at issue time; at rest it is stored as
// an AES-GCM ciphertext keyed by the server secret, so signatures can still be
// verified later without exposing the raw secret in the DB.
type ApiKeyService struct {
	repo   *repository.ApiKeyRepo
	secret string
}

func NewApiKeyService(repo *repository.ApiKeyRepo, secret string) *ApiKeyService {
	return &ApiKeyService{repo: repo, secret: secret}
}

// CreateKey issues a new AK/SK bound to a specific user. permissions is a
// comma-joined permission-code list derived from the user's roles (empty = all
// permissions); expiresAt may be nil.
func (s *ApiKeyService) CreateKey(t, userID uint, name, permissions string, expiresAt *time.Time) (*model.ApiKey, string, error) {
	if strings.TrimSpace(name) == "" {
		return nil, "", errorsBadRequest("name is required")
	}
	if userID == 0 {
		return nil, "", errorsBadRequest("key must be bound to a user")
	}
	ak, err := s.uniqueAK()
	if err != nil {
		return nil, "", err
	}
	sk, err := randomToken(skPrefix, 24)
	if err != nil {
		return nil, "", err
	}
	enc, err := s.encryptSK(sk)
	if err != nil {
		return nil, "", err
	}
	k := &model.ApiKey{
		TenantID:    t,
		UserID:      userID,
		Name:        strings.TrimSpace(name),
		AK:          ak,
		SKCipher:    enc,
		Permissions: strings.TrimSpace(permissions),
		Status:      1,
		ExpiresAt:   expiresAt,
	}
	if err := s.repo.Create(k); err != nil {
		return nil, "", err
	}
	return k, sk, nil
}

// uniqueAK generates an access key and retries on the (negligible) chance of a
// collision with an existing AK.
func (s *ApiKeyService) uniqueAK() (string, error) {
	for i := 0; i < 5; i++ {
		ak, err := randomToken(akPrefix, 18)
		if err != nil {
			return "", err
		}
		if existing, _ := s.repo.GetByAK(ak); existing == nil {
			return ak, nil
		}
	}
	return "", errf(ErrConflict, "failed to generate a unique access key")
}

func randomToken(prefix string, n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// Verify authenticates an AK/SK request and resolves it to an actor. The
// signature covers method + path + body, and the timestamp must be within the
// anti-replay window.
func (s *ApiKeyService) Verify(ak, tsStr, sig, method, path, bodyHash string) (*authx.Actor, error) {
	key, err := s.repo.GetByAK(ak)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, errf(ErrUnauthorized, "invalid api key")
	}
	if key.Status != 1 {
		return nil, errf(ErrUnauthorized, "api key disabled")
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, errf(ErrUnauthorized, "api key expired")
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return nil, errf(ErrUnauthorized, "invalid timestamp")
	}
	if d := time.Now().Unix() - ts; d > aksk.TimestampWindow || d < -aksk.TimestampWindow {
		return nil, errf(ErrUnauthorized, "timestamp outside allowed window")
	}
	sk, err := s.decryptSK(key.SKCipher)
	if err != nil {
		return nil, errf(ErrUnauthorized, "invalid api key")
	}
	expected := aksk.Sign(ak, tsStr, method, path, bodyHash, sk)
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(sig))) {
		return nil, errf(ErrUnauthorized, "signature mismatch")
	}
	return keyActor(key), nil
}

// keyActor turns a key record into an actor carrying the key's tenant and its
// granted permission set. An empty set means "all permissions".
func keyActor(key *model.ApiKey) *authx.Actor {
	var perms []string
	if s := strings.TrimSpace(key.Permissions); s != "" {
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				perms = append(perms, p)
			}
		}
	}
	return &authx.Actor{
		Username: key.Name,
		Name:     key.Name,
		TenantID: key.TenantID,
		KeyAuth:  true,
		KeyPerms: perms,
	}
}

// ---- lifecycle ----

func (s *ApiKeyService) List(t, userID uint, in PageInput) ([]model.ApiKey, int64, error) {
	var (
		out   []model.ApiKey
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(t, userID, f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *ApiKeyService) Disable(t, userID, id uint) error {
	k, err := s.repo.Get(t, userID, id)
	if err != nil {
		return err
	}
	if k == nil {
		return errNotFound(id)
	}
	k.Status = 0
	return s.repo.Update(t, k)
}

func (s *ApiKeyService) Enable(t, userID, id uint) error {
	k, err := s.repo.Get(t, userID, id)
	if err != nil {
		return err
	}
	if k == nil {
		return errNotFound(id)
	}
	k.Status = 1
	return s.repo.Update(t, k)
}

func (s *ApiKeyService) Delete(t, userID, id uint) error {
	return s.repo.Delete(t, userID, id)
}

// ---- SK at-rest encryption ----

func (s *ApiKeyService) gcm() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(s.secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *ApiKeyService) encryptSK(sk string) (string, error) {
	g, err := s.gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := g.Seal(nonce, nonce, []byte(sk), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func (s *ApiKeyService) decryptSK(enc string) (string, error) {
	g, err := s.gcm()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	if len(raw) < g.NonceSize() {
		return "", errorsBadRequest("bad ciphertext")
	}
	nonce, ct := raw[:g.NonceSize()], raw[g.NonceSize():]
	pt, err := g.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
