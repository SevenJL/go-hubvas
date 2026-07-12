package auth

import (
	"context"
	"time"

	"github.com/hubvas/internal/domain/identity"
)

type AccountUnitOfWork interface {
	Register(ctx context.Context, user *identity.User, session RefreshSession) error
	ChangePasswordAndRevokeSessions(ctx context.Context, user *identity.User) error
}

type SessionDTO struct {
	ID         string     `json:"id"`
	UserAgent  string     `json:"user_agent"`
	IPAddress  string     `json:"ip_address,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt time.Time  `json:"last_used_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	Current    bool       `json:"current"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=128"`
}
