package settings

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/version"
)

var (
	invalidFilenameChars = regexp.MustCompile(`[^0-9A-Za-z@_\-.]`)

	dashes = regexp.MustCompile(`[\-]+`)
)

// MakeUserDir makes the user's configured scope directory or generated home
// directory according to settings, then returns the scope to store for the user.
func (s *Settings) MakeUserDir(username, userScope, serverRoot string) (string, error) {
	userScope = strings.TrimSpace(userScope)
	userDir := userScope
	generatedUserDir := false
	if s.CreateUserDir && isGeneratedUserDirScope(userScope) {
		username = CleanUsername(username)
		if username == "" || username == "-" || username == "." {
			log.Printf("create user: invalid user for home dir creation: [%s]", username)
			return "", errors.New("invalid user for home dir creation")
		}
		userDir = path.Join(s.userHomeBasePath(), username)
		// "." preserves the legacy File Browser home-only mode. packageR's
		// explicit full-bucket default is "/" and keeps the stored scope at root.
		if userScope == "." {
			userScope = userDir
		} else {
			userScope = "/"
		}
		generatedUserDir = true
	}

	userDir = path.Join("/", userDir)
	userScope = path.Join("/", userScope)

	fs := afero.NewBasePathFs(afero.NewOsFs(), serverRoot)
	if err := fs.MkdirAll(userDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create user home dir: [%s]: %w", userDir, err)
	}
	if generatedUserDir {
		content := fmt.Sprintf("created by package-r %s automatically - please keep!", version.Version)
		if err := afero.WriteFile(fs, path.Join(userDir, ".keep"), []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to create user home dir marker: [%s]: %w", userDir, err)
		}
	}
	return userScope, nil
}

func isGeneratedUserDirScope(userScope string) bool {
	return userScope == "" || userScope == "." || userScope == "/"
}

// CleanUsername makes a username safe for use as one path segment.
func CleanUsername(s string) string {
	// Remove any trailing space to avoid ending on -
	s = strings.Trim(s, " ")
	s = strings.ReplaceAll(s, "..", "")

	// Replace all characters which not in the list `0-9A-Za-z@_\-.` with a dash
	s = invalidFilenameChars.ReplaceAllString(s, "-")

	// Remove any multiple dashes caused by replacements above
	s = dashes.ReplaceAllString(s, "-")
	return s
}
