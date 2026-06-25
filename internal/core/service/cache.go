package service

import (
	"time"

	"cache/internal/core/port"
)

type CacheService struct {
	store port.Cache
}

func NewCacheService(store port.Cache) *CacheService {
	return &CacheService{store: store}
}

func (s *CacheService) Set(key, value string) {
	s.store.Set(key, value)
}

func (s *CacheService) SetWithTTL(key, value string, ttl time.Duration) {
	s.store.SetWithTTL(key, value, ttl)
}

func (s *CacheService) Get(key string) (string, bool) {
	return s.store.Get(key)
}

func (s *CacheService) Remove(key string) {
	s.store.Remove(key)
}
