package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	fbErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

// MethodProxyAuth is used to identify no auth.
const MethodProxyAuth settings.AuthMethod = "proxy"

// ProxyAuth is a proxy implementation of an auther.
type ProxyAuth struct {
	Header        string `json:"header"`
	Mapper        string `json:"mapper"`
	JWTJwksURL    string `json:"jwtJwksURL"`
	JWTIssuer     string `json:"jwtIssuer"`
	JWTAudience   string `json:"jwtAudience"`
	JWTAlgorithms string `json:"jwtAlgorithms"`
	JWTClockSkew  string `json:"jwtClockSkew"`
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func extractClaimValue(claims map[string]interface{}, key string) (string, bool) {
	if strVal, ok := claims[key].(string); ok {
		return strVal, true
	}
	return "", false
}

func extractUnverifiedClaims(header string) (map[string]interface{}, bool) {
	if strings.Count(header, ".") == 2 {
		token, _, err := jwt.NewParser().ParseUnverified(header, jwt.MapClaims{})
		if err != nil {
			log.Printf("Invalid JWT token in proxy auth header")
			return nil, false
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Printf("Invalid JWT claims in proxy auth header")
			return nil, false
		}
		return claims, true
	}

	token, err := base64.StdEncoding.DecodeString(header)
	if err != nil {
		log.Printf("Invalid base64 token in proxy auth header")
		return nil, false
	}
	var claims map[string]interface{}
	err = json.Unmarshal(token, &claims)
	if err != nil {
		log.Printf("Invalid base64 claims in proxy auth header")
		return nil, false
	}
	return claims, true
}

func (a ProxyAuth) strictJWTEnabled() bool {
	return strings.TrimSpace(a.JWTJwksURL) != ""
}

func (a ProxyAuth) claimName() string {
	return strings.TrimLeft(a.Mapper, ".")
}

func (a ProxyAuth) allowedJWTAlgorithms() []string {
	configured := strings.TrimSpace(a.JWTAlgorithms)
	if configured == "" {
		return []string{jwt.SigningMethodRS256.Alg()}
	}

	algorithms := []string{}
	for _, algorithm := range strings.Split(configured, ",") {
		algorithm = strings.TrimSpace(algorithm)
		if algorithm != "" {
			algorithms = append(algorithms, algorithm)
		}
	}
	if len(algorithms) == 0 {
		return []string{jwt.SigningMethodRS256.Alg()}
	}
	return algorithms
}

func (a ProxyAuth) clockSkew() (time.Duration, error) {
	configured := strings.TrimSpace(a.JWTClockSkew)
	if configured == "" {
		return time.Minute, nil
	}
	return time.ParseDuration(configured)
}

func jwtValidationError(format string, args ...interface{}) (map[string]interface{}, bool) {
	log.Printf(format, args...)
	return nil, false
}

func (a ProxyAuth) extractVerifiedJWTClaims(header string) (map[string]interface{}, bool) {
	if strings.Count(header, ".") != 2 {
		return jwtValidationError("Invalid JWT token shape in proxy auth header")
	}
	if a.claimName() == "" {
		return jwtValidationError("Missing auth.mapper claim for strict proxy JWT validation")
	}
	if strings.TrimSpace(a.JWTIssuer) == "" {
		return jwtValidationError("Missing auth.jwt.issuer for strict proxy JWT validation")
	}

	clockSkew, err := a.clockSkew()
	if err != nil {
		return jwtValidationError("Invalid auth.jwt.clock-skew for strict proxy JWT validation: %v", err)
	}

	options := []jwt.ParserOption{
		jwt.WithValidMethods(a.allowedJWTAlgorithms()),
		jwt.WithIssuer(a.JWTIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(clockSkew),
	}
	if strings.TrimSpace(a.JWTAudience) != "" {
		options = append(options, jwt.WithAudience(a.JWTAudience))
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(header, claims, a.jwksKeyfunc(), options...)
	if err != nil || !token.Valid {
		if err != nil {
			return jwtValidationError("Invalid JWT token in proxy auth header: %v", err)
		}
		return jwtValidationError("Invalid JWT token in proxy auth header")
	}

	return claims, true
}

func (a ProxyAuth) jwksKeyfunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		keyID, _ := token.Header["kid"].(string)
		algorithm, _ := token.Header["alg"].(string)
		key, err := a.rsaPublicKeyFromJWKS(keyID, algorithm)
		if err != nil {
			return nil, err
		}
		return key, nil
	}
}

func (a ProxyAuth) rsaPublicKeyFromJWKS(keyID, algorithm string) (*rsa.PublicKey, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(a.JWTJwksURL)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch JWKS: status %d", resp.StatusCode)
	}

	var jwks jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}

	var fallback *rsa.PublicKey
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" {
			continue
		}
		if key.Use != "" && key.Use != "sig" {
			continue
		}
		if key.Alg != "" && algorithm != "" && key.Alg != algorithm {
			continue
		}

		publicKey, err := rsaPublicKeyFromJWK(key)
		if err != nil {
			continue
		}
		if keyID == "" && fallback == nil {
			fallback = publicKey
		}
		if keyID != "" && key.Kid == keyID {
			return publicKey, nil
		}
	}

	if keyID == "" && fallback != nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("matching RSA signing key not found")
}

func rsaPublicKeyFromJWK(key jwkKey) (*rsa.PublicKey, error) {
	modulusBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}

	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, fmt.Errorf("empty RSA exponent")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}

// mapping strategy:
//
// case 1: "" #empty (default)
//
//	returns the header value
//
// case 2: ".<claim>" #i.e. starting with .
//
//	expects header value to be either a JSON or JWT and the claim
//	to exist returns the extracted claim value
//
// case 3: "<static>" #ianything else
//
//	returns the static value
func (a ProxyAuth) Extract(r *http.Request) (string, bool) {
	header := r.Header.Get(a.Header)
	if header == "" {
		return "", false
	}
	if a.Mapper == "" {
		return header, true
	}
	if a.Mapper[0] != '.' {
		return a.Mapper, true
	}
	var claims map[string]interface{}
	var ok bool
	if a.strictJWTEnabled() {
		claims, ok = a.extractVerifiedJWTClaims(header)
	} else {
		claims, ok = extractUnverifiedClaims(header)
	}
	if !ok {
		return "", false
	}
	return extractClaimValue(claims, a.claimName())
}

// Auth authenticates the user via an HTTP header.
func (a ProxyAuth) Auth(r *http.Request, usr users.Store, setting *settings.Settings, srv *settings.Server) (*users.User, error) {
	if a.Header == "" {
		log.Println("Missing auth.header config")
		return nil, fbErrors.ErrInvalidAuthMethod
	}
	username, ok := a.Extract(r)
	if !ok {
		log.Printf("No value can be inferred from header %s with mapper %s", a.Header, a.Mapper)
		return nil, fbErrors.ErrNotExist
	}
	user, err := usr.Get(srv.Root, username)
	if errors.Is(err, fbErrors.ErrNotExist) {
		if setting.Signup {
			return a.createUser(usr, setting, srv, username)
		}
		log.Printf("User %s not found", username)
	}
	return user, err
}

func (a ProxyAuth) createUser(usr users.Store, setting *settings.Settings, srv *settings.Server, username string) (*users.User, error) {
	const passwordSize = 32
	randomPasswordBytes := make([]byte, passwordSize)
	_, err := rand.Read(randomPasswordBytes)
	if err != nil {
		return nil, err
	}

	var hashedRandomPassword string
	hashedRandomPassword, err = users.HashPwd(string(randomPasswordBytes))
	if err != nil {
		return nil, err
	}

	user := &users.User{
		Username:     username,
		Password:     hashedRandomPassword,
		LockPassword: true,
	}
	setting.ApplyUserDefaults(user)

	var userScope string
	userScope, err = setting.MakeUserDir(user.Username, user.Scope, srv.Root)
	if err != nil {
		return nil, err
	}
	user.Scope = userScope

	err = usr.Save(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// LoginPage tells that proxy auth doesn't require a login page.
func (a ProxyAuth) LoginPage() bool {
	return false
}
