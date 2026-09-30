package rclonefs

import (
	"context"
	"fmt"
	"strings"

	rclone "github.com/rclone/rclone/fs"
)

// SetLogLevel validates and applies the shared process log level to rclone.
func SetLogLevel(value string) error {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	switch value {
	case "ERROR", "NOTICE", "INFO", "DEBUG":
	default:
		return fmt.Errorf("invalid rclone log level %q; use ERROR, NOTICE, INFO, or DEBUG", value)
	}

	level := rclone.LogLevelNotice
	if err := level.Set(value); err != nil {
		return fmt.Errorf("invalid rclone log level: %w", err)
	}

	config := rclone.GetConfig(context.Background())
	config.LogLevel = level
	if err := config.Reload(context.Background()); err != nil {
		return fmt.Errorf("apply rclone log level: %w", err)
	}
	return nil
}
