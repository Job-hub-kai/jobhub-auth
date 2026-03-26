package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/job-hub-kai/jobhub-auth/internal/config"
	"github.com/job-hub-kai/jobhub-auth/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	registerTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_register_total",
		Help: "Total number of registration attempts",
	}, []string{"status"})

	loginTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_login_total",
		Help: "Total number of login attempts",
	}, []string{"status"})
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
}

type TokenRepository interface {
	SaveRefreshToken(ctx context.Context, token *domain.Token) error
	GetRefreshToken(ctx context.Context, token string) (*domain.Token, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteAllUserTokens(ctx context.Context, userID string) error
	BlacklistAccessToken(ctx context.Context, token string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
}

type AccessClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

type AuthService struct {
	users  UserRepository
	tokens TokenRepository
	cfg    config.JWTConfig
	log    *zap.Logger
}

func NewAuthService(users UserRepository, tokens TokenRepository, cfg config.JWTConfig, log *zap.Logger) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
		cfg:    cfg,
		log:    log,
	}
}

func (s *AuthService) Register(ctx context.Context, input domain.RegisterInput) (string, error) {
	_, err := s.users.GetByEmail(ctx, input.Email)
	if err == nil {
		registerTotal.WithLabelValues("failure").Inc()
		return "", domain.ErrUserAlreadyExists
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		registerTotal.WithLabelValues("failure").Inc()
		s.log.Error("register: get by email", zap.Error(err))
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		registerTotal.WithLabelValues("failure").Inc()
		s.log.Error("register: bcrypt", zap.Error(err))
		return "", err
	}

	now := time.Now()
	user := &domain.User{
		ID:           uuid.New().String(),
		Email:        input.Email,
		Name:         input.Name,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		registerTotal.WithLabelValues("failure").Inc()
		s.log.Error("register: create user", zap.Error(err))
		return "", err
	}

	registerTotal.WithLabelValues("success").Inc()
	return user.ID, nil
}

func (s *AuthService) Login(ctx context.Context, input domain.LoginInput) (*domain.TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, input.Email)
	if err != nil {
		loginTotal.WithLabelValues("failure").Inc()
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrUserNotFound
		}
		s.log.Error("login: get by email", zap.Error(err))
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		loginTotal.WithLabelValues("failure").Inc()
		return nil, domain.ErrInvalidPassword
	}

	pair, err := s.generateTokenPair(user.ID, user.Email)
	if err != nil {
		loginTotal.WithLabelValues("failure").Inc()
		s.log.Error("login: generate tokens", zap.Error(err))
		return nil, err
	}

	if err := s.saveRefreshToken(ctx, user.ID, pair.RefreshToken); err != nil {
		loginTotal.WithLabelValues("failure").Inc()
		s.log.Error("login: save refresh token", zap.Error(err))
		return nil, err
	}

	loginTotal.WithLabelValues("success").Inc()
	return pair, nil
}

func (s *AuthService) Logout(ctx context.Context, accessToken string) error {
	claims, err := s.parseAccessToken(accessToken)
	if err != nil {
		return domain.ErrInvalidToken
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 {
		if err := s.tokens.BlacklistAccessToken(ctx, accessToken, ttl); err != nil {
			s.log.Error("logout: blacklist token", zap.Error(err))
			return err
		}
	}

	if err := s.tokens.DeleteAllUserTokens(ctx, claims.UserID); err != nil {
		s.log.Error("logout: delete refresh tokens", zap.Error(err))
		return err
	}

	return nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	stored, err := s.tokens.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	if time.Now().After(stored.ExpiresAt) {
		_ = s.tokens.DeleteRefreshToken(ctx, refreshToken)
		return nil, domain.ErrTokenExpired
	}

	claims, err := s.parseRefreshToken(refreshToken)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	if err := s.tokens.DeleteRefreshToken(ctx, refreshToken); err != nil {
		s.log.Error("refresh: delete old token", zap.Error(err))
		return nil, err
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		s.log.Error("refresh: get user", zap.Error(err))
		return nil, err
	}

	pair, err := s.generateTokenPair(user.ID, user.Email)
	if err != nil {
		s.log.Error("refresh: generate tokens", zap.Error(err))
		return nil, err
	}

	if err := s.saveRefreshToken(ctx, user.ID, pair.RefreshToken); err != nil {
		s.log.Error("refresh: save new token", zap.Error(err))
		return nil, err
	}

	return pair, nil
}

func (s *AuthService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, accessToken string) (string, string, error) {
	claims, err := s.parseAccessToken(accessToken)
	if err != nil {
		return "", "", domain.ErrInvalidToken
	}

	blacklisted, err := s.tokens.IsBlacklisted(ctx, accessToken)
	if err != nil {
		s.log.Error("validate: check blacklist", zap.Error(err))
		return "", "", err
	}
	if blacklisted {
		return "", "", domain.ErrTokenBlacklisted
	}

	return claims.UserID, claims.Email, nil
}

func (s *AuthService) generateTokenPair(userID, email string) (*domain.TokenPair, error) {
	now := time.Now()

	accessClaims := &AccessClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.AccessSecret))
	if err != nil {
		return nil, err
	}

	refreshClaims := &RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.RefreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.RefreshSecret))
	if err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) saveRefreshToken(ctx context.Context, userID, tokenStr string) error {
	t := &domain.Token{
		ID:        uuid.New().String(),
		UserID:    userID,
		Token:     tokenStr,
		ExpiresAt: time.Now().Add(s.cfg.RefreshTTL),
		CreatedAt: time.Now(),
	}
	return s.tokens.SaveRefreshToken(ctx, t)
}

func (s *AuthService) parseAccessToken(tokenStr string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidToken
		}
		return []byte(s.cfg.AccessSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrInvalidToken
	}
	return claims, nil
}

func (s *AuthService) parseRefreshToken(tokenStr string) (*RefreshClaims, error) {
	claims := &RefreshClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidToken
		}
		return []byte(s.cfg.RefreshSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrInvalidToken
	}
	return claims, nil
}
