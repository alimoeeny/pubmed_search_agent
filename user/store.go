// Package user provides user profile storage backed by Supabase Postgres.
package user

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SubscriptionPlan represents the billing tier for a user.
type SubscriptionPlan string

const (
	PlanFree SubscriptionPlan = "free"
	PlanPro  SubscriptionPlan = "pro"
	PlanMax  SubscriptionPlan = "max"
)

// UserProfile holds the persisted profile for an authenticated user.
type UserProfile struct {
	UserID       string
	Email        string
	Plan         SubscriptionPlan
	Enabled      bool
	ModelDefault string
	PDFStyle     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Store persists and retrieves user profiles from Postgres.
type Store interface {
	// Upsert inserts a new profile or updates the email on conflict.
	// Called on every authenticated request to auto-provision new users.
	Upsert(ctx context.Context, userID, email string) (UserProfile, error)
	// Get retrieves a profile by user ID.
	Get(ctx context.Context, userID string) (UserProfile, error)
}

// PostgresStore implements Store using a pgxpool connection pool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgresStore backed by the provided pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

const scanColumns = `user_id, email, plan, enabled,
	COALESCE(model_default, ''), COALESCE(pdf_style, ''),
	created_at, updated_at`

func scanProfile(row interface{ Scan(...any) error }) (UserProfile, error) {
	var p UserProfile
	var plan string
	err := row.Scan(
		&p.UserID, &p.Email, &plan, &p.Enabled,
		&p.ModelDefault, &p.PDFStyle,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return UserProfile{}, err
	}
	p.Plan = SubscriptionPlan(plan)
	return p, nil
}

// Upsert implements Store.
func (s *PostgresStore) Upsert(ctx context.Context, userID, email string) (UserProfile, error) {
	const q = `
		INSERT INTO user_profiles (user_id, email)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
			SET email      = EXCLUDED.email,
			    updated_at = now()
		RETURNING ` + scanColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID, email))
	if err != nil {
		return UserProfile{}, fmt.Errorf("user store: upsert: %w", err)
	}
	return p, nil
}

// Get implements Store.
func (s *PostgresStore) Get(ctx context.Context, userID string) (UserProfile, error) {
	const q = `SELECT ` + scanColumns + ` FROM user_profiles WHERE user_id = $1`

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID))
	if err != nil {
		return UserProfile{}, fmt.Errorf("user store: get: %w", err)
	}
	return p, nil
}
