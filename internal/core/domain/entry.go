package domain

import "time"

type CacheEntry struct {
	Value     string
	ExpiresAt time.Time
}

func (e CacheEntry) IsExpired() bool {
	return !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt)
}
