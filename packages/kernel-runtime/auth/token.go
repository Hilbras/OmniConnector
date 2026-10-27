package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
)

// Token represents an access token
type Token struct {
	Value     string
	Type      string
	ExpiresAt time.Time
	Scopes    []string
}

// TokenManager manages token lifecycle
type TokenManager struct {
	tokens map[string]*Token
}

// New creates a new token manager
func New() *TokenManager {
	return &TokenManager{tokens: make(map[string]*Token)}
}

// Generate creates a new token
func (tm *TokenManager) Generate(scopes []string, ttl time.Duration) (*Token, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	token := &Token{
		Value:     base64.URLEncoding.EncodeToString(b),
		Type:      "bearer",
		ExpiresAt: time.Now().Add(ttl),
		Scopes:    scopes,
	}
	tm.tokens[token.Value] = token
	return token, nil
}

// Validate checks whether a token is valid and not expired
func (tm *TokenManager) Validate(value string) (*Token, error) {
	token, ok := tm.tokens[value]
	if !ok {
		return nil, errors.New("token not found")
	}
	if time.Now().After(token.ExpiresAt) {
		delete(tm.tokens, value)
		return nil, errors.New("token expired")
	}
	return token, nil
}

// Revoke removes a token
func (tm *TokenManager) Revoke(value string) {
	delete(tm.tokens, value)
}

// RevokeAll removes all tokens for a given scope
func (tm *TokenManager) RevokeAll() {
	tm.tokens = make(map[string]*Token)
}

// Count returns the number of active tokens
func (tm *TokenManager) Count() int {
	return len(tm.tokens)
}
