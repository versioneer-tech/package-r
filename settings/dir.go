package settings

import (
	"errors"
	"log"
	"path"
	"regexp"
	"strings"
)

var (
	invalidFilenameChars = regexp.MustCompile(`[^0-9A-Za-z@_\-.]`)

	dashes = regexp.MustCompile(`[\-]+`)
)

// ResolveUserScope returns the object-storage scope to store for a user.
// Object directories are created through the user's rclone VFS when data is
// written. This function must not create paths below the local server root.
func (s *Settings) ResolveUserScope(username, userScope string) (string, error) {
	userScope = strings.TrimSpace(userScope)
	if s.CreateUserDir && isGeneratedUserDirScope(userScope) {
		username = CleanUsername(username)
		if username == "" || username == "-" || username == "." {
			log.Printf("create user: invalid user for home scope: [%s]", username)
			return "", errors.New("invalid user for home scope")
		}
		// "." preserves the inherited home-only mode. packageR's
		// explicit full-bucket default is "/" and keeps the stored scope at root.
		if userScope == "." {
			userScope = path.Join(s.userHomeBasePath(), username)
		} else {
			userScope = "/"
		}
	}

	return path.Join("/", userScope), nil
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
