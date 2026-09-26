package rclonefs

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

func TestWrappedStoreAppliesContainedBasePath(t *testing.T) {
	base := afero.NewMemMapFs()
	if err := afero.WriteFile(base, "/team/alice/report.txt", []byte("alice"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(base, "/team/alice2/secret.txt", []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	original := &fakeUserStore{
		user: &users.User{ID: 1, Username: "alice", Scope: "/team/alice"},
	}
	wrapped := WrapUsers(original, &staticProvider{fileSystem: base}, nil)

	user, err := wrapped.Get("/ignored-local-root", uint(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := user.Fs.(*afero.BasePathFs); !ok {
		t.Fatalf("expected BasePathFs, got %T", user.Fs)
	}
	data, err := afero.ReadFile(user.Fs, "/report.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "alice" {
		t.Fatalf("unexpected scoped data %q", data)
	}
	if err := afero.WriteFile(user.Fs, "/new.txt", []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if exists, err := afero.Exists(base, "/team/alice/new.txt"); err != nil || !exists {
		t.Fatalf("write did not stay in user scope: exists=%v err=%v", exists, err)
	}

	for _, name := range []string{"../alice2/secret.txt", "/../alice2/secret.txt"} {
		if _, err := user.Fs.Open(name); !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("expected %q to be denied, got %v", name, err)
		}
	}
}

func TestWrappedStoreSupportsNestedShareScopeAndGets(t *testing.T) {
	base := afero.NewMemMapFs()
	if err := afero.WriteFile(base, "/team/alice/shared/note.txt", []byte("note"), 0644); err != nil {
		t.Fatal(err)
	}
	original := &fakeUserStore{
		user: &users.User{ID: 1, Username: "alice", Scope: "/team/alice"},
		all: []*users.User{
			{ID: 1, Username: "alice", Scope: "/team/alice"},
			{ID: 2, Username: "bob", Scope: "/team/bob"},
		},
	}
	wrapped := WrapUsers(original, &staticProvider{fileSystem: base}, nil)

	user, err := wrapped.Get("", uint(1))
	if err != nil {
		t.Fatal(err)
	}
	shareFS := afero.NewBasePathFs(user.Fs, "/shared")
	data, err := afero.ReadFile(shareFS, "/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "note" {
		t.Fatalf("unexpected shared data %q", data)
	}

	all, err := wrapped.Gets("")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range all {
		if _, ok := item.Fs.(*afero.BasePathFs); !ok {
			t.Fatalf("expected user %d to have BasePathFs, got %T", item.ID, item.Fs)
		}
	}
}

func TestWrappedStorePublicLinkIncludesUserScope(t *testing.T) {
	provider := &staticProvider{
		fileSystem: afero.NewMemMapFs(),
		publicURL:  "https://objects.example.invalid/report.txt",
	}
	wrapped := WrapUsers(&fakeUserStore{}, provider, nil)
	linker, ok := wrapped.(users.PublicLinker)
	if !ok {
		t.Fatal("wrapped user store does not provide public links")
	}

	user := &users.User{
		Scope: "/team/alice",
	}
	link, err := linker.PublicLink(context.Background(), user, "/files/report.txt", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if link != provider.publicURL {
		t.Fatalf("unexpected public link %q", link)
	}
	if provider.publicName != "/team/alice/files/report.txt" {
		t.Fatalf("unexpected public link path %q", provider.publicName)
	}
	if provider.publicExpire != time.Hour {
		t.Fatalf("unexpected public link expiry %v", provider.publicExpire)
	}
}

func TestWrappedStoreCreatesUserHomeWhenEnabled(t *testing.T) {
	t.Setenv("PACKAGE_R_ROOT", "reports")
	base := afero.NewMemMapFs()
	original := &fakeUserStore{
		user: &users.User{ID: 1, Username: "alice", Scope: "/"},
	}
	wrapped := WrapUsers(original, &staticProvider{fileSystem: base}, &settings.Settings{
		CreateUserDir: true,
	})

	if _, err := wrapped.Get("", uint(1)); err != nil {
		t.Fatal(err)
	}
	if exists, err := afero.DirExists(base, "/home/alice"); err != nil || !exists {
		t.Fatalf("expected generated user home: exists=%v err=%v", exists, err)
	}
	if exists, err := afero.Exists(base, "/home/alice/.keep"); err != nil || !exists {
		t.Fatalf("expected persistent generated user home marker: exists=%v err=%v", exists, err)
	}
}

func TestWrappedStoreDoesNotCreateAdminHome(t *testing.T) {
	t.Setenv("PACKAGE_R_ROOT", "reports")
	base := afero.NewMemMapFs()
	original := &fakeUserStore{
		user: &users.User{
			ID:       1,
			Username: "admin",
			Scope:    "/",
			Perm:     users.Permissions{Admin: true},
		},
	}
	wrapped := WrapUsers(original, &staticProvider{fileSystem: base}, &settings.Settings{
		CreateUserDir: true,
	})

	if _, err := wrapped.Get("", uint(1)); err != nil {
		t.Fatal(err)
	}
	if exists, err := afero.DirExists(base, "/home/admin"); err != nil || exists {
		t.Fatalf("expected no generated admin home: exists=%v err=%v", exists, err)
	}
}

type staticProvider struct {
	fileSystem   afero.Fs
	err          error
	publicURL    string
	publicName   string
	publicExpire time.Duration
}

func (p *staticProvider) FileSystem() (afero.Fs, error) {
	return p.fileSystem, p.err
}

func (p *staticProvider) PublicLink(_ context.Context, name string, expire time.Duration) (string, error) {
	p.publicName = name
	p.publicExpire = expire
	return p.publicURL, p.err
}

type fakeUserStore struct {
	user *users.User
	all  []*users.User
}

func (s *fakeUserStore) Get(string, interface{}) (*users.User, error) {
	copy := *s.user
	return &copy, nil
}

func (s *fakeUserStore) Gets(string) ([]*users.User, error) {
	result := make([]*users.User, 0, len(s.all))
	for _, user := range s.all {
		copy := *user
		result = append(result, &copy)
	}
	return result, nil
}

func (s *fakeUserStore) Update(*users.User, ...string) error { return nil }
func (s *fakeUserStore) Save(*users.User) error              { return nil }
func (s *fakeUserStore) Delete(interface{}) error            { return nil }
func (s *fakeUserStore) LastUpdate(uint) int64               { return 0 }
