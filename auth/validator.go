package auth

import "github.com/google/uuid"

// Validator wraps Auth to implement the AuthValidator interface for HTTP handlers.
type Validator struct {
	auth *Auth
}

// NewValidator creates a new Validator.
func NewValidator(auth *Auth) *Validator {
	return &Validator{auth: auth}
}

// ValidateToken validates a JWT token and returns the user ID.
func (v *Validator) ValidateToken(token string) (uuid.UUID, error) {
	claims, err := v.auth.ValidateToken(token)
	if err != nil {
		return uuid.Nil, err
	}
	return claims.UserID, nil
}
