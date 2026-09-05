package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"scm/internal/model"
	"scm/pkg/authx"
	"scm/internal/repo"
)

// tokenVersion guards against stale tokens issued before multi-tenancy:
// without roles/tid claims they would render the app empty (menu gating) and
// 403 on every route, instead of cleanly bouncing to login.
const tokenVersion = "2"

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
		"ver":      tokenVersion, // reject pre-multitenancy tokens
		"iat":      now.Unix(),
		"exp":      now.Add(s.ttl).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
	if err != nil {
		return "", nil, nil, err
	}
	return token, u, roles, nil
}

// ChangePassword verifies the old password and replaces it with newPassword.
func (s *AuthService) ChangePassword(actor *authx.Actor, oldPassword, newPassword string) error {
	if actor == nil || actor.UserID == 0 {
		return errf(ErrUnauthorized, "login required")
	}
	if len(newPassword) < 6 {
		return errorsBadRequest("new password must be at least 6 characters")
	}
	u, err := s.users.Get(actor.TenantID, actor.UserID)
	if err != nil {
		return err
	}
	if u == nil {
		return errf(ErrUnauthorized, "user not found")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)) != nil {
		return errf(ErrUnauthorized, "old password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return s.users.Update(actor.TenantID, u)
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
	if ver, _ := mc["ver"].(string); ver != tokenVersion {
		return nil, errf(ErrUnauthorized, "session expired, please sign in again")
	}
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
