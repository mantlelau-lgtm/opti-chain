package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"scm/internal/model"
	"scm/internal/pkg/authx"
	"scm/internal/repository"
)

// AuthService handles tenant-aware login and token issuing/parsing.
type AuthService struct {
	users     *repository.UserRepo
	tenants   *repository.TenantRepo
	userRoles *repository.UserRoleRepo
	secret    string
	ttl       time.Duration
}

func NewAuthService(users *repository.UserRepo, tenants *repository.TenantRepo, userRoles *repository.UserRoleRepo, secret string, ttl time.Duration) *AuthService {
	return &AuthService{users: users, tenants: tenants, userRoles: userRoles, secret: secret, ttl: ttl}
}

// Login verifies tenant + credentials and issues a signed HS256 token whose
// claims carry the tenant id and role codes.
func (s *AuthService) Login(username, password, tenantCode string) (string, *model.User, []string, error) {
	tenant, err := s.tenants.GetByCode(tenantCode)
	if err != nil {
		return "", nil, nil, err
	}
	if tenant == nil || tenant.Status != model.TenantActive {
		return "", nil, nil, errf(ErrUnauthorized, "invalid tenant or tenant suspended")
	}
	u, err := s.users.GetByTenantUsername(tenant.ID, username)
	if err != nil {
		return "", nil, nil, err
	}
	if u == nil || u.Status != 1 ||
		bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", nil, nil, errf(ErrUnauthorized, "invalid username or password")
	}
	roles, err := s.userRoles.RoleCodesForUser(u.ID)
	if err != nil {
		return "", nil, nil, err
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"uid":      u.ID,
		"username": u.Username,
		"name":     u.Name,
		"tid":      tenant.ID,
		"roles":    roles,
		"iat":      now.Unix(),
		"exp":      now.Add(s.ttl).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
	if err != nil {
		return "", nil, nil, err
	}
	return token, u, roles, nil
}

// ParseToken validates a bearer token and extracts the actor.
func (s *AuthService) ParseToken(token string) (*authx.Actor, error) {
	t, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.secret), nil
	})
	if err != nil || !t.Valid {
		return nil, errf(ErrUnauthorized, "invalid or expired token")
	}
	mc, _ := t.Claims.(jwt.MapClaims)
	uid, _ := mc["uid"].(float64)
	tid, _ := mc["tid"].(float64)
	username, _ := mc["username"].(string)
	name, _ := mc["name"].(string)
	var roles []string
	if rs, ok := mc["roles"].([]any); ok {
		for _, r := range rs {
			if rc, ok := r.(string); ok {
				roles = append(roles, rc)
			}
		}
	}
	return &authx.Actor{UserID: uint(uid), Username: username, Name: name, TenantID: uint(tid), Roles: roles}, nil
}
