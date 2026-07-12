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
	accounts    AccountUnitOfWork
	tokenSvc    TokenService
	passwordSvc PasswordService
	refreshTTL  time.Duration // absolute lifetime of a refresh-session family
}

func NewAuthApplicationService(userRepo identity.UserRepository, sessions RefreshSessionRepository, accounts AccountUnitOfWork, tokenSvc TokenService, passwordSvc PasswordService, refreshTTL time.Duration) *AuthApplicationService {
	return &AuthApplicationService{userRepo: userRepo, sessions: sessions, accounts: accounts, tokenSvc: tokenSvc, passwordSvc: passwordSvc, refreshTTL: refreshTTL}
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
	hash, err := s.passwordSvc.Hash(req.Password)
	if err != nil {
		return nil, err
	}
	user, err := identity.NewUser(0, req.Username, req.Email, hash)
	if err != nil {
		return nil, err
	}
	session, refresh, err := newRefreshCredential(0, "", s.refreshTTL, meta)
	if err != nil {
		return nil, err
	}
	if s.accounts == nil {
		return nil, shared.NewDomainError(shared.ErrConflict, "transactional account registration is unavailable")
	}
	if err = s.accounts.Register(ctx, user, session); err != nil {
		return nil, err
	}
	// The access token must carry the database-assigned user ID. Generate it
	// only after the transaction commits; refresh-session creation is already
	// durable and can be revoked if token generation unexpectedly fails.
	access, expires, err := s.tokenSvc.GenerateAccessToken(user.ID())
	if err != nil {
		_ = s.sessions.RevokeAll(ctx, user.ID())
		return nil, err
	}
	tokens := &TokenResponse{AccessToken: access, RefreshToken: refresh, ExpiresAt: expires, TokenType: "Bearer"}
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

func (s *AuthApplicationService) Sessions(ctx context.Context, userID identity.UserID, currentToken string) ([]SessionDTO, error) {
	var currentHash []byte
	if currentToken != "" {
		currentHash = hashRefreshToken(currentToken)
	}
	return s.sessions.List(ctx, userID, currentHash)
}

func (s *AuthApplicationService) RevokeSession(ctx context.Context, userID identity.UserID, sessionID string) error {
	if sessionID == "" {
		return shared.NewDomainError(shared.ErrInvalidArgument, "session id is required")
	}
	return s.sessions.RevokeByID(ctx, userID, sessionID)
}

func (s *AuthApplicationService) ChangePassword(ctx context.Context, userID identity.UserID, req ChangePasswordRequest) error {
	if req.CurrentPassword == "" || len(req.NewPassword) < 8 || len(req.NewPassword) > 128 {
		return shared.NewDomainError(shared.ErrInvalidArgument, "password must be 8-128 characters")
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !s.passwordSvc.Verify(req.CurrentPassword, user.PasswordHash()) {
		return shared.NewDomainError(shared.ErrUnauthorized, "current password is incorrect")
	}
	if s.passwordSvc.Verify(req.NewPassword, user.PasswordHash()) {
		return shared.NewDomainError(shared.ErrConflict, "new password must be different")
	}
	hash, err := s.passwordSvc.Hash(req.NewPassword)
	if err != nil {
		return err
	}
	if err = user.ChangePassword(hash); err != nil {
		return err
	}
	if s.accounts == nil {
		return shared.NewDomainError(shared.ErrConflict, "transactional password change is unavailable")
	}
	return s.accounts.ChangePasswordAndRevokeSessions(ctx, user)
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
