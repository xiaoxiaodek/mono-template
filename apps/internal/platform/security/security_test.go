package security

import (
	"testing"
	"time"
)

func TestPasswordHasherVerifiesHash(t *testing.T) {
	hasher := BcryptPasswordHasher{Cost: 10}

	hash, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !hasher.Compare(hash, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if hasher.Compare(hash, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestJWTManagerSignsAndVerifiesAccessToken(t *testing.T) {
	manager := NewJWTManager("test-secret-with-enough-length", 15*time.Minute, time.Hour)

	token, err := manager.SignAccessToken(Principal{
		UserID: "usr_1",
		Email:  "a@example.com",
		Roles:  []string{"admin"},
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := manager.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != "usr_1" || claims.TokenType != "access" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestJWTManagerRejectsRefreshTokenAsAccessToken(t *testing.T) {
	manager := NewJWTManager("test-secret-with-enough-length", 15*time.Minute, time.Hour)
	token, err := manager.SignRefreshToken(Principal{UserID: "usr_1"})
	if err != nil {
		t.Fatalf("sign refresh token: %v", err)
	}

	if _, err := manager.VerifyAccessToken(token); err == nil {
		t.Fatal("expected refresh token to be rejected as an access token")
	}
}
