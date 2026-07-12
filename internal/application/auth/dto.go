package auth

import "time"

// RegisterRequest is the input DTO for user registration.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// LoginRequest is the input DTO for user login.
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest is the input DTO for refreshing tokens.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// TokenResponse is the output DTO containing JWT tokens.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	TokenType    string `json:"token_type"` // "Bearer"
}

// UserDTO is the public representation of a user.
type UserDTO struct {
	ID          int64     `json:"id,string"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	Website     string    `json:"website"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	AccountRole string    `json:"account_role"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UpdateProfileRequest is the input DTO for updating user profile.
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name" binding:"required,min=1,max=50"`
	Bio         string `json:"bio" binding:"max=500"`
	Website     string `json:"website" binding:"max=2048"`
}

// RegisterResponse is the combined response for registration (user + tokens auto-login).
type RegisterResponse struct {
	User   *UserDTO       `json:"user"`
	Tokens *TokenResponse `json:"tokens"`
}
