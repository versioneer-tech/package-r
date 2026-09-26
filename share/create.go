package share

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	Path   string
	UserID uint
}

func NewLink(body CreateBody, opts LinkOptions) (*Link, error) {
	hash, err := resolveHash(body.Hash)
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
	if body.Expiration != "" {
		expire, err = compactExpiration(body.Expiration, time.Now())
		if err != nil {
			return nil, err
		}
	}

	passwordHash, token, err := getPasswordAuth(body.Password)
	if err != nil {
		return nil, err
	}

	catalogURL := ""
	if body.CatalogName != "" {
		catalogURL, err = CatalogPath(opts.Path, body.CatalogName)
		if err != nil {
			return nil, err
		}
	}
	if err := ValidateCatalogAssetMappings(body.AssetMappings); err != nil {
		return nil, err
	}

	return &Link{
		Path:          opts.Path,
		Hash:          hash,
		Expire:        expire,
		Description:   body.Description,
		CatalogURL:    catalogURL,
		AssetMappings: append([]CatalogAssetMapping(nil), body.AssetMappings...),
		UserID:        opts.UserID,
		PasswordHash:  passwordHash,
		Token:         token,
	}, nil
}

// ParseCatalogAssetMappings parses a JSON array of catalog asset mappings.
func ParseCatalogAssetMappings(value string) ([]CatalogAssetMapping, error) {
	if value == "" {
		return nil, nil
	}

	var mappings []CatalogAssetMapping
	if err := json.Unmarshal([]byte(value), &mappings); err != nil {
		return nil, err
	}
	if err := ValidateCatalogAssetMappings(mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

// ValidateCatalogAssetMappings checks that mappings stay inside the share.
func ValidateCatalogAssetMappings(mappings []CatalogAssetMapping) error {
	for i, mapping := range mappings {
		if mapping.From == "" {
			return fmt.Errorf("catalog asset mapping %d has an empty from value", i)
		}
		if !catalogMappingPathIsRelative(mapping.To) {
			return fmt.Errorf("catalog asset mapping %d to value must stay inside the share", i)
		}
	}
	return nil
}

func catalogMappingPathIsRelative(value string) bool {
	if strings.HasPrefix(value, "/") || strings.ContainsRune(value, '\x00') {
		return false
	}
	clean := path.Clean(value)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

func CatalogPath(sharePath, catalogName string) (string, error) {
	if catalogName == "" {
		return "", nil
	}
	if strings.HasPrefix(catalogName, "/") {
		return "", fmt.Errorf("catalog name must be relative: %s", catalogName)
	}
	if strings.ContainsRune(catalogName, '\x00') {
		return "", fmt.Errorf("catalog name contains a NUL byte")
	}

	cleanName := path.Clean(catalogName)
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "../") {
		return "", fmt.Errorf("catalog name must stay within the shared tree: %s", catalogName)
	}

	return path.Join("/", sharePath, cleanName), nil
}

func resolveHash(hash string) (string, error) {
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
	return randomHash, nil
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

func compactExpiration(value string, now time.Time) (int64, error) {
	if value == "" {
		return 0, nil
	}
	if len(value) < 2 {
		return 0, invalidExpiration(value)
	}

	number := value[:len(value)-1]
	for _, digit := range number {
		if digit < '0' || digit > '9' {
			return 0, invalidExpiration(value)
		}
	}
	amount, err := strconv.Atoi(number)
	if err != nil || amount <= 0 {
		return 0, invalidExpiration(value)
	}

	var expires time.Time
	switch value[len(value)-1] {
	case 'd':
		expires = now.AddDate(0, 0, amount)
	case 'm':
		expires = now.AddDate(0, amount, 0)
	case 'H':
		const maxHours = int64(^uint64(0)>>1) / int64(time.Hour)
		if int64(amount) > maxHours {
			return 0, invalidExpiration(value)
		}
		expires = now.Add(time.Duration(amount) * time.Hour)
	case 'y':
		expires = now.AddDate(amount, 0, 0)
	default:
		return 0, invalidExpiration(value)
	}

	return expires.Unix(), nil
}

func invalidExpiration(value string) error {
	return fmt.Errorf("invalid expiration %q: use <number>d, <number>m, <number>H, or <number>y", value)
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
