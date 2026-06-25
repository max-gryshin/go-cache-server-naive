package port

import "time"

type Cache interface {
	Set(key, value string)
	SetWithTTL(key, value string, ttl time.Duration)
	Get(key string) (string, bool)
	Remove(key string)
}
