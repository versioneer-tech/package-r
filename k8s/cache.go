package k8s

import (
	"errors"
	"fmt"
	"time"

	"github.com/bluele/gcache"
	v1 "k8s.io/api/core/v1"
)

type Cache struct {
	secrets gcache.Cache
}

const (
	cacheSize = 1000
	cacheTTL  = 60 * time.Second
)

func NewCache() *Cache {
	return &Cache{
		secrets: gcache.New(cacheSize).Expiration(cacheTTL).ARC().Build(),
	}
}

func (cache *Cache) GetSecret(name string, loader func(string) (*v1.Secret, error)) (*v1.Secret, error) {
	val, err := cache.secrets.Get(name)
	if err != nil {
		if errors.Is(err, gcache.KeyNotFoundError) {
			if loader != nil {
				secret, err2 := loader(name)
				if err2 != nil {
					return nil, fmt.Errorf("loader error: %w", err2)
				}
				err3 := cache.secrets.SetWithExpire(secret.Name, secret, cacheTTL)
				if err3 != nil {
					return nil, fmt.Errorf("cache update error: %w", err3)
				}

				return secret, nil
			}
		}

		return nil, fmt.Errorf("get: %w", err)
	}

	return val.(*v1.Secret), nil
}
