package auth

import (
	"context"
	"time"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type TokenService interface {
	GenerateAccessToken(userID identity.UserID) (string, int64, error)
	ValidateAccessToken(tokenString string) (identity.UserID, error)
}
type PasswordService interface {
	Hash(password string) (string, error)
	Verify(password, hash string) bool
}

type AuthApplicationService struct {
	userRepo    identity.UserRepository
	sessions    RefreshSessionRepository
	tokenSvc    TokenService
	passwordSvc PasswordService
	refreshTTL  time.Duration // absolute lifetime of a refresh-session family
}

func NewAuthApplicationService(userRepo identity.UserRepository, sessions RefreshSessionRepository, tokenSvc TokenService, passwordSvc PasswordService, refreshTTL time.Duration) *AuthApplicationService {
	return &AuthApplicationService{userRepo: userRepo, sessions: sessions, tokenSvc: tokenSvc, passwordSvc: passwordSvc, refreshTTL: refreshTTL}
}

func (s *AuthApplicationService) issue(ctx context.Context, userID identity.UserID, family string, meta SessionMetadata) (*TokenResponse, error) {
	access, expires, err := s.tokenSvc.GenerateAccessToken(userID)
	if err != nil {
		return nil, err
	}
	session, refresh, err := newRefreshCredential(userID, family, s.refreshTTL, meta)
	if err != nil {
		return nil, err
	}
	if err = s.sessions.Create(ctx, session); err != nil {
		return nil, err
	}
	return &TokenResponse{AccessToken: access, RefreshToken: refresh, ExpiresAt: expires, TokenType: "Bearer"}, nil
}

func (s *AuthApplicationService) Register(ctx context.Context, req RegisterRequest, meta SessionMetadata) (*RegisterResponse, error) {
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, shared.NewDomainError(shared.ErrAlreadyExists, "username is taken")
	}
	exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, shared.NewDomainError(shared.ErrAlreadyExists, "email is already registered")
	}
	hash, err := s.passwordSvc.Hash(req.Password)
	if err != nil {
		return nil, err
	}
	user, err := identity.NewUser(0, req.Username, req.Email, hash)
	if err != nil {
		return nil, err
	}
	if err = s.userRepo.Save(ctx, user); err != nil {
		return nil, err
	}
	tokens, err := s.issue(ctx, user.ID(), "", meta)
	if err != nil {
		return nil, err
	}
	return &RegisterResponse{User: userDTO(user), Tokens: tokens}, nil
}

func (s *AuthApplicationService) Login(ctx context.Context, req LoginRequest, meta SessionMetadata) (*TokenResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil || !s.passwordSvc.Verify(req.Password, user.PasswordHash()) {
		return nil, shared.NewDomainError(shared.ErrUnauthorized, "invalid email or password")
	}
	if !user.IsActive() {
		return nil, shared.NewDomainError(shared.ErrForbidden, "account is suspended")
	}
	return s.issue(ctx, user.ID(), "", meta)
}

func (s *AuthApplicationService) Refresh(ctx context.Context, token string, meta SessionMetadata) (*TokenResponse, error) {
	if token == "" {
		return nil, shared.NewDomainError(shared.ErrUnauthorized, "refresh cookie is required")
	}
	next, raw, err := newRefreshCredential(0, "", s.refreshTTL, meta)
	if err != nil {
		return nil, err
	}
	userID, err := s.sessions.Rotate(ctx, hashRefreshToken(token), next)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || !user.IsActive() {
		_ = s.sessions.RevokeAll(ctx, userID)
		return nil, shared.NewDomainError(shared.ErrUnauthorized, "account is unavailable")
	}
	access, expires, err := s.tokenSvc.GenerateAccessToken(userID)
	if err != nil {
		return nil, err
	}
	return &TokenResponse{AccessToken: access, RefreshToken: raw, ExpiresAt: expires, TokenType: "Bearer"}, nil
}
func (s *AuthApplicationService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.sessions.Revoke(ctx, hashRefreshToken(token))
}
func (s *AuthApplicationService) LogoutAll(ctx context.Context, userID identity.UserID) error {
	return s.sessions.RevokeAll(ctx, userID)
}

func (s *AuthApplicationService) UpdateProfile(ctx context.Context, userID identity.UserID, req UpdateProfileRequest) (*UserDTO, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err = user.UpdateProfile(req.DisplayName, req.Bio, req.Website); err != nil {
		return nil, err
	}
	if err = s.userRepo.Save(ctx, user); err != nil {
		return nil, err
	}
	return userDTO(user), nil
}
func (s *AuthApplicationService) GetUser(ctx context.Context, userID identity.UserID) (*UserDTO, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return userDTO(user), nil
}
func userDTO(user *identity.User) *UserDTO {
	return &UserDTO{ID: int64(user.ID()), Username: user.Username(), Email: user.Email(), DisplayName: user.DisplayName(), Bio: user.Bio(), Website: user.Website(), AvatarURL: user.AvatarURL(), AccountRole: user.AccountRole(), Status: user.Status(), CreatedAt: user.CreatedAt(), UpdatedAt: user.UpdatedAt()}
}
