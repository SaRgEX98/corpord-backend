package token

import (
	"corpord-api/internal/config"
	"corpord-api/model"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenRejectsWeakSecretAndInvalidRegisteredClaims(t *testing.T) {
	for _, secret := range []string{"", "short"} {
		m := NewManager(&config.JWT{Secret: secret, SigningAlgorithm: "HS256"})
		if _, err := m.Generate(GenerateParams{UserID: 1}); err == nil {
			t.Fatal("weak signing secret accepted")
		}
		if _, err := m.Validate("anything"); err == nil {
			t.Fatal("weak validation secret accepted")
		}
	}
	secret := strings.Repeat("s", 32)
	m := NewManager(&config.JWT{Secret: secret, SigningAlgorithm: "HS256", AccessTokenTTL: time.Minute})
	raw, err := m.Generate(GenerateParams{UserID: 1, Role: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Validate(raw); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, issuer, audience string
		expiry                 *jwt.NumericDate
	}{
		{"wrong issuer", "other", "corpord-web", jwt.NewNumericDate(time.Now().Add(time.Minute))},
		{"wrong audience", "corpord-api", "other", jwt.NewNumericDate(time.Now().Add(time.Minute))},
		{"missing expiry", "corpord-api", "corpord-web", nil},
		{"expired", "corpord-api", "corpord-web", jwt.NewNumericDate(time.Now().Add(-time.Minute))},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := &model.Claims{UserID: 1, RegisteredClaims: jwt.RegisteredClaims{Issuer: test.issuer, Audience: []string{test.audience}, ExpiresAt: test.expiry}}
			raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.Validate(raw); err == nil {
				t.Fatal("invalid registered claims accepted")
			}
		})
	}
}
