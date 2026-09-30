// Package logging controls optional packageR log messages.
package logging

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

const (
	infoDurationThreshold   = 3 * time.Second
	noticeDurationThreshold = 10 * time.Second
)

type level int32

const (
	levelError level = iota
	levelNotice
	levelInfo
	levelDebug
)

var configuredLevel atomic.Int32

func init() {
	configuredLevel.Store(int32(levelNotice))
}

// SetLevel validates and applies the packageR log level.
// An empty value keeps the current level.
func SetLevel(value string) error {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return nil
	}

	levels := map[string]level{
		"ERROR":  levelError,
		"NOTICE": levelNotice,
		"INFO":   levelInfo,
		"DEBUG":  levelDebug,
	}
	selected, ok := levels[value]
	if !ok {
		return fmt.Errorf("invalid log level %q; use ERROR, NOTICE, INFO, or DEBUG", value)
	}
	configuredLevel.Store(int32(selected))
	return nil
}

func enabled(messageLevel level) bool {
	return level(configuredLevel.Load()) >= messageLevel
}

// Debugf logs a packageR debug message when debug logging is enabled.
func Debugf(format string, args ...any) {
	if !enabled(levelDebug) {
		return
	}
	log.Printf("DEBUG: "+format, args...)
}

// Infof logs a packageR informational message when info logging is enabled.
func Infof(format string, args ...any) {
	if !enabled(levelInfo) {
		return
	}
	log.Printf("INFO: "+format, args...)
}

// Noticef logs an important packageR operational message.
func Noticef(format string, args ...any) {
	if !enabled(levelNotice) {
		return
	}
	log.Printf("NOTICE: "+format, args...)
}

// Timedf logs a timed operation at debug, info, or notice according to its
// elapsed duration.
func Timedf(elapsed time.Duration, format string, args ...any) {
	switch {
	case elapsed > noticeDurationThreshold:
		Noticef(format, args...)
	case elapsed > infoDurationThreshold:
		Infof(format, args...)
	default:
		Debugf(format, args...)
	}
}
