package share

import (
	"errors"
	"testing"
	"time"

	appErrors "github.com/versioneer-tech/package-r/errors"
)

type readOnlyExpirationBackend struct {
	links []*Link
}

func (b *readOnlyExpirationBackend) All() ([]*Link, error) {
	return append([]*Link(nil), b.links...), nil
}

func (b *readOnlyExpirationBackend) GetByHash(hash string) (*Link, error) {
	for _, link := range b.links {
		if link.Hash == hash {
			return link, nil
		}
	}
	return nil, appErrors.ErrNotExist
}

func (b *readOnlyExpirationBackend) GetPermanent(string, uint) (*Link, error) {
	return nil, appErrors.ErrNotExist
}

func (b *readOnlyExpirationBackend) Save(*Link) error   { return nil }
func (b *readOnlyExpirationBackend) Update(*Link) error { return nil }

func TestExpirationChecksDoNotNeedStorageDeletion(t *testing.T) {
	expired := &Link{Hash: "expired", Expire: time.Now().Add(-time.Minute).Unix()}
	active := &Link{Hash: "active", Expire: time.Now().Add(time.Minute).Unix()}
	backend := &readOnlyExpirationBackend{links: []*Link{expired, active}}
	storage := NewStorage(backend)

	links, err := storage.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Hash != active.Hash {
		t.Fatalf("expected only the active share, got %#v", links)
	}
	if _, err := storage.GetByHash(expired.Hash); !errors.Is(err, appErrors.ErrNotExist) {
		t.Fatalf("expected an expired share to be unavailable, got %v", err)
	}
	if len(backend.links) != 2 {
		t.Fatalf("expected expiration reads not to mutate storage, got %#v", backend.links)
	}
}
