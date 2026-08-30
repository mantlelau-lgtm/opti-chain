package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/pkg/authx"
	"scm/internal/repository"
)

// AuthService handles login, token issuing/parsing and the bootstrap admin.
type AuthService struct {
	users  *repository.UserRepo
	secret string
	ttl    time.Duration
	db     *gorm.DB
}

func NewAuthService(users *repository.UserRepo, secret string, ttl time.Duration, db *gorm.DB) *AuthService {
	return &AuthService{users: users, secret: secret, ttl: ttl, db: db}
}

// SeedAdmin creates the default admin (admin/admin123) when the user table is
// empty, so a fresh deployment is immediately usable.
func (s *AuthService) SeedAdmin() error {
	var count int64
	if err := s.db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.Create(&model.User{
		Username:     "admin",
		PasswordHash: string(hash),
		Name:         "管理员",
		Status:       1,
	})
}

// Login verifies credentials and issues a signed HS256 token.
func (s *AuthService) Login(username, password string) (string, *model.User, error) {
	u, err := s.users.GetByUsername(username)
	if err != nil {
		return "", nil, err
	}
	if u == nil || u.Status != 1 ||
		bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", nil, errf(ErrUnauthorized, "invalid username or password")
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"uid":      u.ID,
		"username": u.Username,
		"name":     u.Name,
		"iat":      now.Unix(),
		"exp":      now.Add(s.ttl).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
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
	username, _ := mc["username"].(string)
	name, _ := mc["name"].(string)
	return &authx.Actor{UserID: uint(uid), Username: username, Name: name}, nil
}
