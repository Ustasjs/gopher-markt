package service

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTService_RoundTrip(t *testing.T) {
	svc := NewJWTService([]byte("test-secret"))

	token, err := svc.GenerateToken("user-42")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := svc.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-42" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-42")
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Before(time.Now()) {
		t.Errorf("ExpiresAt = %v, want future date", claims.ExpiresAt)
	}
}

func TestJWTService_ParseUserID(t *testing.T) {
	svc := NewJWTService([]byte("test-secret"))

	token, err := svc.GenerateToken("user-7")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	userID, err := svc.ParseUserID(token)
	if err != nil {
		t.Fatalf("ParseUserID: %v", err)
	}
	if userID != "user-7" {
		t.Errorf("userID = %q, want %q", userID, "user-7")
	}
}

func TestJWTService_ParseUserID_InvalidToken(t *testing.T) {
	svc := NewJWTService([]byte("test-secret"))

	if _, err := svc.ParseUserID("not-a-token"); err == nil {
		t.Error("expected error for invalid token, got nil")
	}
}

func TestJWTService_ParseToken_WrongSecret(t *testing.T) {
	signer := NewJWTService([]byte("secret-A"))
	verifier := NewJWTService([]byte("secret-B"))

	token, err := signer.GenerateToken("user-1")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if _, err := verifier.ParseToken(token); err == nil {
		t.Error("expected error when verifying with different secret, got nil")
	}
}

func TestJWTService_ParseToken_Expired(t *testing.T) {
	secret := []byte("test-secret")
	expiredClaims := Claims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	tokenString, err := expiredToken.SignedString(secret)
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	svc := NewJWTService(secret)
	if _, err := svc.ParseToken(tokenString); err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestJWTService_ParseToken_NonHMACAlgorithm(t *testing.T) {
	claims := Claims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}

	svc := NewJWTService([]byte("test-secret"))
	_, err = svc.ParseToken(tokenString)
	if err == nil {
		t.Fatal("expected error for non-HMAC token, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected signing method") &&
		!strings.Contains(err.Error(), "failed to parse token") {
		t.Errorf("err = %v, want signing method rejection", err)
	}
}

func TestJWTService_ParseToken_Garbage(t *testing.T) {
	svc := NewJWTService([]byte("test-secret"))

	cases := []string{
		"",
		"not.a.jwt",
		"only-one-segment",
		"two.segments",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			if _, err := svc.ParseToken(tc); err == nil {
				t.Errorf("ParseToken(%q) = nil error, want error", tc)
			}
		})
	}
}
