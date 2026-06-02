package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// BcryptPasswordService implements application/auth.PasswordService using bcrypt.
type BcryptPasswordService struct {
	cost int
}

// NewBcryptPasswordService creates the service with the given bcrypt cost.
// Default cost of 12 is a good balance for production.
func NewBcryptPasswordService(cost int) *BcryptPasswordService {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptPasswordService{cost: cost}
}

// Hash generates a bcrypt hash of the password.
func (s *BcryptPasswordService) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Verify compares a password against a bcrypt hash.
func (s *BcryptPasswordService) Verify(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
