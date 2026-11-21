package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestProxyAuthMapperUsesDocumentedSingleDotClaim(t *testing.T) {
	claims := map[string]string{"sub": "alice"}
	encodedClaims, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", http.NoBody)
	req.Header.Set("X-Userinfo", base64.StdEncoding.EncodeToString(encodedClaims))

	got, ok := (&ProxyAuth{Header: "X-Userinfo", Mapper: ".sub"}).Extract(req)
	if !ok {
		t.Fatal("expected proxy mapper to extract claim")
	}
	if got != "alice" {
		t.Fatalf("expected username alice, got %q", got)
	}
}

func TestProxyAuthKeepsTrustedProxyUnverifiedJWTDecode(t *testing.T) {
	token := signedProxyTestJWT(t, &proxyTestTokenConfig{
		PrivateKey: proxyTestRSAKey(t),
		KeyID:      "unverified",
		Issuer:     "https://issuer.example/",
		Audience:   "package-r",
		Subject:    "alice",
		ExpiresAt:  time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest("GET", "/", http.NoBody)
	req.Header.Set("X-Access-Token", token)

	got, ok := (&ProxyAuth{Header: "X-Access-Token", Mapper: ".preferred_username"}).Extract(req)
	if !ok {
		t.Fatal("expected trusted-proxy mode to decode JWT claim without signature validation")
	}
	if got != "alice" {
		t.Fatalf("expected username alice, got %q", got)
	}
}

func TestProxyAuthReportsUnverifiedClaimMapping(t *testing.T) {
	tests := []struct {
		name string
		auth ProxyAuth
		want bool
	}{
		{name: "trusted username header", auth: ProxyAuth{Mapper: ""}},
		{name: "unverified claim", auth: ProxyAuth{Mapper: ".sub"}, want: true},
		{name: "verified claim", auth: ProxyAuth{Mapper: ".sub", JWTJwksURL: "https://identity.example/jwks.json"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.auth.UsesUnverifiedClaimMapping(); got != test.want {
				t.Fatalf("expected %t, got %t", test.want, got)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "standard", value: "Bearer token", want: "token"},
		{name: "lowercase", value: "bearer token", want: "token"},
		{name: "raw token", value: "token", want: "token"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := bearerToken(test.value); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestProxyAuthValidatesStrictJWTViaJWKS(t *testing.T) {
	key := proxyTestRSAKey(t)
	jwks := proxyTestJWKSServer(t, key, "key-1")

	token := signedProxyTestJWT(t, &proxyTestTokenConfig{
		PrivateKey: key,
		KeyID:      "key-1",
		Issuer:     "https://issuer.example/",
		Audience:   "package-r",
		Subject:    "alice",
		ExpiresAt:  time.Now().Add(time.Hour),
		NotBefore:  time.Now().Add(-time.Minute),
	})

	for _, header := range []string{"X-Access-Token", "Authorization"} {
		t.Run(header, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", http.NoBody)
			value := token
			if header == "Authorization" {
				value = "Bearer " + token
			}
			req.Header.Set(header, value)
			got, ok := (&ProxyAuth{
				Header:        header,
				Mapper:        ".preferred_username",
				JWTJwksURL:    jwks.URL,
				JWTIssuer:     "https://issuer.example/",
				JWTAudience:   "package-r",
				JWTAlgorithms: "RS256",
				JWTClockSkew:  "30s",
			}).Extract(req)
			if !ok || got != "alice" {
				t.Fatalf("validated username=%q accepted=%t", got, ok)
			}
		})
	}
}

func TestProxyAuthStrictJWTRejectsInvalidTokens(t *testing.T) {
	key := proxyTestRSAKey(t)
	jwks := proxyTestJWKSServer(t, key, "key-1")

	tests := []struct {
		name  string
		token string
	}{
		{
			name: "wrong signature",
			token: signedProxyTestJWT(t, &proxyTestTokenConfig{
				PrivateKey: proxyTestRSAKey(t),
				KeyID:      "key-1",
				Issuer:     "https://issuer.example/",
				Audience:   "package-r",
				Subject:    "alice",
				ExpiresAt:  time.Now().Add(time.Hour),
			}),
		},
		{
			name: "wrong issuer",
			token: signedProxyTestJWT(t, &proxyTestTokenConfig{
				PrivateKey: key,
				KeyID:      "key-1",
				Issuer:     "https://other-issuer.example/",
				Audience:   "package-r",
				Subject:    "alice",
				ExpiresAt:  time.Now().Add(time.Hour),
			}),
		},
		{
			name: "wrong audience",
			token: signedProxyTestJWT(t, &proxyTestTokenConfig{
				PrivateKey: key,
				KeyID:      "key-1",
				Issuer:     "https://issuer.example/",
				Audience:   "other-service",
				Subject:    "alice",
				ExpiresAt:  time.Now().Add(time.Hour),
			}),
		},
		{
			name: "expired",
			token: signedProxyTestJWT(t, &proxyTestTokenConfig{
				PrivateKey: key,
				KeyID:      "key-1",
				Issuer:     "https://issuer.example/",
				Audience:   "package-r",
				Subject:    "alice",
				ExpiresAt:  time.Now().Add(-time.Hour),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", http.NoBody)
			req.Header.Set("X-Access-Token", tt.token)

			got, ok := (&ProxyAuth{
				Header:        "X-Access-Token",
				Mapper:        ".preferred_username",
				JWTJwksURL:    jwks.URL,
				JWTIssuer:     "https://issuer.example/",
				JWTAudience:   "package-r",
				JWTAlgorithms: "RS256",
				JWTClockSkew:  "30s",
			}).Extract(req)
			if ok {
				t.Fatalf("expected strict proxy JWT validation to fail, got username %q", got)
			}
		})
	}
}

type proxyTestTokenConfig struct {
	PrivateKey *rsa.PrivateKey
	KeyID      string
	Issuer     string
	Audience   string
	Subject    string
	ExpiresAt  time.Time
	NotBefore  time.Time
}

func signedProxyTestJWT(t *testing.T, cfg *proxyTestTokenConfig) string {
	t.Helper()

	claims := jwt.MapClaims{
		"iss":                cfg.Issuer,
		"aud":                cfg.Audience,
		"exp":                jwt.NewNumericDate(cfg.ExpiresAt),
		"preferred_username": cfg.Subject,
	}
	if !cfg.NotBefore.IsZero() {
		claims["nbf"] = jwt.NewNumericDate(cfg.NotBefore)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = cfg.KeyID
	signed, err := token.SignedString(cfg.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func proxyTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func proxyTestJWKSServer(t *testing.T, key *rsa.PrivateKey, keyID string) *httptest.Server {
	t.Helper()

	body, err := json.Marshal(map[string]interface{}{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": keyID,
				"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}
