package rclonefs

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	rclone "github.com/rclone/rclone/fs"

	appLogging "github.com/versioneer-tech/package-r/logging"
)

func TestSetLogLevel(t *testing.T) {
	config := rclone.GetConfig(context.Background())
	previous := config.LogLevel
	t.Cleanup(func() {
		config.LogLevel = previous
		_ = config.Reload(context.Background())
	})

	if err := SetLogLevel("debug"); err != nil {
		t.Fatal(err)
	}
	if config.LogLevel != rclone.LogLevelDebug {
		t.Fatalf("unexpected rclone log level %q", config.LogLevel)
	}
	if err := SetLogLevel("invalid"); err == nil {
		t.Fatal("expected an invalid rclone log level error")
	}
	if err := SetLogLevel("WARNING"); err == nil {
		t.Fatal("expected an unsupported operator log level error")
	}
}

func TestListLogUsesDurationLevel(t *testing.T) {
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
		_ = appLogging.SetLevel("NOTICE")
	})

	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetFlags(0)
	if err := appLogging.SetLevel("DEBUG"); err != nil {
		t.Fatal(err)
	}

	logList("fast", 1, time.Second, nil)
	logList("slow", 2, 3*time.Second+time.Millisecond, nil)
	logList("very-slow", 3, 10*time.Second+time.Millisecond, nil)

	message := output.String()
	for _, expected := range []string{
		`DEBUG: rclone list path="fast"`,
		`INFO: rclone list path="slow"`,
		`NOTICE: rclone list path="very-slow"`,
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("list log does not contain %q: %q", expected, message)
		}
	}
}
