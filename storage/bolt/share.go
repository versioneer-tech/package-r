package bolt

import (
	"errors"

	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"

	fbErrors "github.com/versioneer-tech/package-r/v2/errors"
	"github.com/versioneer-tech/package-r/v2/share"
)

type shareBackend struct {
	db *storm.DB
}

func (s shareBackend) All() ([]*share.Link, error) {
	var v []*share.Link
	err := s.db.All(&v)
	if errors.Is(err, storm.ErrNotFound) {
		return v, fbErrors.ErrNotExist
	}

	return v, err
}

func (s shareBackend) FindByUserID(id uint) ([]*share.Link, error) {
	var v []*share.Link
	err := s.db.Select(q.Eq("UserID", id)).Find(&v)
	if errors.Is(err, storm.ErrNotFound) {
		return v, fbErrors.ErrNotExist
	}

	return v, err
}

func (s shareBackend) GetByHash(hash string) (*share.Link, error) {
	var v share.Link
	err := s.db.One("Hash", hash, &v)
	if errors.Is(err, storm.ErrNotFound) {
		return nil, fbErrors.ErrNotExist
	}

	return &v, err
}

func (s shareBackend) GetPermanent(path string, id uint) (*share.Link, error) {
	var v share.Link
	err := s.db.Select(q.Eq("Path", path), q.Eq("Expire", 0), q.Eq("UserID", id)).First(&v)
	if errors.Is(err, storm.ErrNotFound) {
		return nil, fbErrors.ErrNotExist
	}

	return &v, err
}

func (s shareBackend) Gets(path, sourceName string, id uint) ([]*share.Link, error) {
	var v []*share.Link
	err := s.db.Select(q.Eq("Path", path), q.Eq("UserID", id)).Find(&v)
	if errors.Is(err, storm.ErrNotFound) {
		return v, fbErrors.ErrNotExist
	}
	if err != nil {
		return v, err
	}
	var filteredv = make([]*share.Link, 0)
	for _, s := range v {
		if s.Source.Name == sourceName {
			filteredv = append(filteredv, s)
		}
	}
	return filteredv, nil
}

func (s shareBackend) Save(l *share.Link) error {
	return s.db.Save(l)
}

func (s shareBackend) Delete(hash string) error {
	err := s.db.DeleteStruct(&share.Link{Hash: hash})
	if errors.Is(err, storm.ErrNotFound) {
		return nil
	}
	return err
}
