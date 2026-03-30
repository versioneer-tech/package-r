package share

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// HashPattern limits custom share hashes to URL-safe lowercase values.
var HashPattern = regexp.MustCompile(`^[a-z0-9.-]{1,20}$`)

type LinkOptions struct {
	Path           string
	UserID         uint
	DefaultHash    string
	CatalogBaseURL string
}

func NewLink(body CreateBody, opts LinkOptions) (*Link, error) {
	hash, err := resolveHash(body.Hash, opts.DefaultHash)
	if err != nil {
		return nil, err
	}

	if !HashPattern.MatchString(hash) {
		return nil, fmt.Errorf("invalid hash: %s", hash)
	}

	expire, err := getExpire(body.Expires, body.Unit)
	if err != nil {
		return nil, err
	}

	passwordHash, token, err := getPasswordAuth(body.Password)
	if err != nil {
		return nil, err
	}

	catalogURL := ""
	if opts.CatalogBaseURL != "" && body.CatalogName != "" {
		catalogURL = path.Join(opts.CatalogBaseURL, opts.Path, body.CatalogName)
	}

	return &Link{
		Path:          opts.Path,
		Hash:          hash,
		Expire:        expire,
		Description:   body.Description,
		CatalogURL:    catalogURL,
		FiltersField:  body.FiltersField,
		AssetsBaseURL: body.AssetsBaseURL,
		UserID:        opts.UserID,
		PasswordHash:  passwordHash,
		Token:         token,
	}, nil
}

func resolveHash(hash string, defaultHash string) (string, error) {
	if hash != "" {
		return hash, nil
	}

	const charset = "abcdefghjkmnpqrstuvwxyz23456789" // no 0, O, l, 1, I
	random := make([]byte, 8)
	for i := range random {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		random[i] = charset[n.Int64()]
	}

	randomHash := string(random)
	if defaultHash == "" {
		return randomHash, nil
	}

	if strings.Contains(defaultHash, "<random>") {
		return strings.Replace(defaultHash, "<random>", randomHash, 1), nil
	}

	return defaultHash + randomHash, nil
}

func getExpire(expires string, unit string) (int64, error) {
	if expires == "" {
		return 0, nil
	}

	num, err := strconv.Atoi(expires)
	if err != nil {
		return 0, err
	}

	var add time.Duration
	switch unit {
	case "seconds":
		add = time.Second * time.Duration(num)
	case "minutes":
		add = time.Minute * time.Duration(num)
	case "days":
		add = time.Hour * 24 * time.Duration(num)
	default:
		add = time.Hour * time.Duration(num)
	}

	return time.Now().Add(add).Unix(), nil
}

func getPasswordAuth(password string) (string, string, error) {
	if password == "" {
		return "", "", nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("failed to hash password: %w", err)
	}

	tokenBuffer := make([]byte, 96)
	if _, err := rand.Read(tokenBuffer); err != nil {
		return "", "", err
	}

	return string(hash), base64.URLEncoding.EncodeToString(tokenBuffer), nil
}
