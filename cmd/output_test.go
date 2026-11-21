package cmd

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/versioneer-tech/package-r/auth"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

func TestCLIOutputReportsWriteErrors(t *testing.T) {
	tests := []struct {
		name   string
		failOn string
		write  func(io.Writer) error
	}{
		{
			name: "user table flush",
			write: func(output io.Writer) error {
				return writeUsers(output, []*users.User{{Username: "test-user"}})
			},
		},
		{
			name: "settings table",
			write: func(output io.Writer) error {
				return writeSettings(output, &settings.Server{}, &settings.Settings{}, &auth.JSONAuth{})
			},
		},
		{
			name:   "authentication output",
			failOn: "\nAuthor configuration (raw):",
			write: func(output io.Writer) error {
				return writeSettings(output, &settings.Server{}, &settings.Settings{}, &auth.JSONAuth{})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.write(failingOutput{failOn: test.failOn})
			if !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("expected output failure, got %v", err)
			}
		})
	}
}

type failingOutput struct {
	failOn string
}

func (w failingOutput) Write(data []byte) (int, error) {
	if w.failOn == "" || strings.HasPrefix(string(data), w.failOn) {
		return 0, io.ErrClosedPipe
	}
	return len(data), nil
}
