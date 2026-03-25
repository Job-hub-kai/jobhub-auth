package redis

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/job-hub-kai/jobhub-auth/internal/domain"
	"github.com/redis/go-redis/v9"
)

type TokenRepository struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func NewTokenRepository(db *pgxpool.Pool, rdb *redis.Client) *TokenRepository {
	return &TokenRepository{db: db, rdb: rdb}
}

func (r *TokenRepository) SaveRefreshToken(ctx context.Context, token *domain.Token) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.Exec(ctx, query,
		token.ID,
		token.UserID,
		token.Token,
		token.ExpiresAt,
		token.CreatedAt,
	)
	return err
}

func (r *TokenRepository) GetRefreshToken(ctx context.Context, token string) (*domain.Token, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM refresh_tokens
		WHERE token = $1`

	t := &domain.Token{}
	err := r.db.QueryRow(ctx, query, token).Scan(
		&t.ID,
		&t.UserID,
		&t.Token,
		&t.ExpiresAt,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}
	return t, nil
}

func (r *TokenRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	query := `DELETE FROM refresh_tokens WHERE token = $1`
	_, err := r.db.Exec(ctx, query, token)
	return err
}

func (r *TokenRepository) DeleteAllUserTokens(ctx context.Context, userID string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`
	_, err := r.db.Exec(ctx, query, userID)
	return err
}

func (r *TokenRepository) BlacklistAccessToken(ctx context.Context, token string, ttl time.Duration) error {
	return r.rdb.Set(ctx, blacklistKey(token), 1, ttl).Err()
}

func (r *TokenRepository) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	err := r.rdb.Get(ctx, blacklistKey(token)).Err()
	if err == nil {
		return true, nil
	}
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return false, err
}

func blacklistKey(token string) string {
	return "blacklist:" + token
}
