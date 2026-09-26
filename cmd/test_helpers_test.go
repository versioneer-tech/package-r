package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"

	"github.com/versioneer-tech/package-r/storage"
	"github.com/versioneer-tech/package-r/storage/bolt"
)

func TestPackageRCommandHelper(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}

		rootCmd.SetArgs(os.Args[i+1:])
		if err := rootCmd.Execute(); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
}

func runPackageRCommand(t *testing.T, args ...string) {
	t.Helper()

	output, err := runPackageRCommandResult(t, args...)
	if err != nil {
		t.Fatalf("packageR command failed: %v\n%s", err, output)
	}
}

func runPackageRCommandResult(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), os.Args[0], append([]string{"-test.run=^TestPackageRCommandHelper$", "--"}, args...)...)
	return cmd.CombinedOutput()
}

func newConfigTestDB(t *testing.T) (dbPath string, configPath string, rootPath string) {
	t.Helper()

	dir := t.TempDir()
	return filepath.Join(dir, "package-r.db"),
		filepath.Join(dir, "missing-config.yaml"),
		filepath.Join(dir, "root")
}

func openTestStorage(t *testing.T, dbPath string) *storage.Storage {
	t.Helper()

	db, err := storm.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})

	store := bolt.NewStorage(db)
	return store
}
