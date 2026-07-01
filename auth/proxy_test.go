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

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Userinfo", base64.StdEncoding.EncodeToString(encodedClaims))

	got, ok := ProxyAuth{Header: "X-Userinfo", Mapper: ".sub"}.Extract(req)
	if !ok {
		t.Fatal("expected proxy mapper to extract claim")
	}
	if got != "alice" {
		t.Fatalf("expected username alice, got %q", got)
	}
}

func TestProxyAuthKeepsTrustedProxyUnverifiedJWTDecode(t *testing.T) {
	token := signedProxyTestJWT(t, proxyTestTokenConfig{
		PrivateKey: proxyTestRSAKey(t),
		KeyID:      "unverified",
		Issuer:     "https://issuer.example/",
		Audience:   "package-r",
		Subject:    "alice",
		ExpiresAt:  time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Access-Token", token)

	got, ok := ProxyAuth{Header: "X-Access-Token", Mapper: ".preferred_username"}.Extract(req)
	if !ok {
		t.Fatal("expected trusted-proxy mode to decode JWT claim without signature validation")
	}
	if got != "alice" {
		t.Fatalf("expected username alice, got %q", got)
	}
}

func TestProxyAuthExtractsSelfCreatedJWTWithJWKSURL(t *testing.T) {
	key := proxyTestRSAKey(t)
	jwks := proxyTestJWKSServer(t, key, "self-created-key")

	token := signedProxyTestJWT(t, proxyTestTokenConfig{
		PrivateKey: key,
		KeyID:      "self-created-key",
		Issuer:     "https://issuer.example/",
		Audience:   "workspace-api",
		Subject:    "service-account-package-r",
		ExpiresAt:  time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", token)

	got, ok := ProxyAuth{
		Header:      "Authorization",
		Mapper:      ".preferred_username",
		JWTJwksURL:  jwks.URL,
		JWTIssuer:   "https://issuer.example/",
		JWTAudience: "workspace-api",
	}.Extract(req)
	if !ok {
		t.Fatal("expected self-created JWT to pass strict JWKS validation")
	}
	if got != "service-account-package-r" {
		t.Fatalf("expected service account username, got %q", got)
	}
}

func TestProxyAuthExtractsSelfCreatedJWTWithoutJWKSURL(t *testing.T) {
	token := signedProxyTestJWT(t, proxyTestTokenConfig{
		PrivateKey: proxyTestRSAKey(t),
		KeyID:      "trusted-proxy-key",
		Issuer:     "https://issuer.example/",
		Audience:   "workspace-api",
		Subject:    "service-account-package-r",
		ExpiresAt:  time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", token)

	got, ok := ProxyAuth{
		Header: "Authorization",
		Mapper: ".preferred_username",
	}.Extract(req)
	if !ok {
		t.Fatal("expected self-created JWT to be decoded in trusted-proxy mode")
	}
	if got != "service-account-package-r" {
		t.Fatalf("expected service account username, got %q", got)
	}
}

func TestProxyAuthValidatesStrictJWTViaJWKS(t *testing.T) {
	key := proxyTestRSAKey(t)
	jwks := proxyTestJWKSServer(t, key, "key-1")

	token := signedProxyTestJWT(t, proxyTestTokenConfig{
		PrivateKey: key,
		KeyID:      "key-1",
		Issuer:     "https://issuer.example/",
		Audience:   "package-r",
		Subject:    "alice",
		ExpiresAt:  time.Now().Add(time.Hour),
		NotBefore:  time.Now().Add(-time.Minute),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Access-Token", token)

	got, ok := ProxyAuth{
		Header:        "X-Access-Token",
		Mapper:        ".preferred_username",
		JWTJwksURL:    jwks.URL,
		JWTIssuer:     "https://issuer.example/",
		JWTAudience:   "package-r",
		JWTAlgorithms: "RS256",
		JWTClockSkew:  "30s",
	}.Extract(req)
	if !ok {
		t.Fatal("expected strict proxy JWT validation to pass")
	}
	if got != "alice" {
		t.Fatalf("expected username alice, got %q", got)
	}
}

func TestProxyAuthStrictJWTRejectsInvalidTokens(t *testing.T) {
	key := proxyTestRSAKey(t)
	jwks := proxyTestJWKSServer(t, key, "key-1")

	tests := []struct {
		name  string
		token string
		auth  ProxyAuth
	}{
		{
			name: "wrong signature",
			token: signedProxyTestJWT(t, proxyTestTokenConfig{
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
			token: signedProxyTestJWT(t, proxyTestTokenConfig{
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
			token: signedProxyTestJWT(t, proxyTestTokenConfig{
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
			token: signedProxyTestJWT(t, proxyTestTokenConfig{
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
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Access-Token", tt.token)

			got, ok := ProxyAuth{
				Header:        "X-Access-Token",
				Mapper:        ".preferred_username",
				JWTJwksURL:    jwks.URL,
				JWTIssuer:     "https://issuer.example/",
				JWTAudience:   "package-r",
				JWTAlgorithms: "RS256",
				JWTClockSkew:  "30s",
			}.Extract(req)
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

func signedProxyTestJWT(t *testing.T, cfg proxyTestTokenConfig) string {
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

	return httptest.NewServer(httpHandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

type httpHandlerFunc func(http.ResponseWriter, *http.Request)

func (f httpHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}
