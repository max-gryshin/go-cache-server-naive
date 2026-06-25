package in_memory

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"cache/internal/core/domain"
)

const evictionInterval = time.Second

type Store struct {
	mu        sync.RWMutex
	data      map[string]domain.CacheEntry
	expiry    *expiryHeap
	heapItems map[string]*heapItem
	lru       *lruList
	sizeUsed  int64
	sizeLimit int64
}

func NewStore(sizeLimit int64) *Store {
	return &Store{
		data:      make(map[string]domain.CacheEntry),
		expiry:    newExpiryHeap(),
		heapItems: make(map[string]*heapItem),
		lru:       newLRUList(),
		sizeLimit: sizeLimit,
	}
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.write(key, domain.CacheEntry{Value: value})
}

func (s *Store) SetWithTTL(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiresAt := time.Now().Add(ttl)
	entry := domain.CacheEntry{Value: value, ExpiresAt: expiresAt}
	s.write(key, entry)
	item := &heapItem{key: key, expiresAt: expiresAt}
	heap.Push(s.expiry, item)
	s.heapItems[key] = item
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.data[key]
	if !ok {
		return "", false
	}
	if entry.IsExpired() {
		s.delete(key)
		return "", false
	}
	s.lru.touch(key)
	return entry.Value, true
}

func (s *Store) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delete(key)
}

func (s *Store) StartEviction(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(evictionInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.evictExpired()
			}
		}
	}()
}

// write sets a key in the map, updates LRU and size accounting, then
// evicts LRU entries until the size limit is satisfied.
// Must be called with s.mu held.
func (s *Store) write(key string, entry domain.CacheEntry) {
	if old, exists := s.data[key]; exists {
		s.sizeUsed -= entrySize(key, old.Value)
		s.tombstone(key)
	}
	s.data[key] = entry
	s.lru.add(key)
	s.sizeUsed += entrySize(key, entry.Value)
	for s.sizeLimit > 0 && s.sizeUsed > s.sizeLimit {
		s.evictLRU()
	}
}

// evictLRU removes the least recently used entry.
// Must be called with s.mu held.
func (s *Store) evictLRU() {
	key := s.lru.evict()
	if key == "" {
		return
	}
	entry := s.data[key]
	s.sizeUsed -= entrySize(key, entry.Value)
	s.tombstone(key)
	delete(s.data, key)
}

// evictExpired pops expired entries from the heap and removes them from the store.
func (s *Store) evictExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for {
		item := s.expiry.peek()
		if item == nil {
			break
		}
		if item.deleted {
			heap.Pop(s.expiry)
			continue
		}
		if item.expiresAt.After(now) {
			break
		}
		heap.Pop(s.expiry)
		entry := s.data[item.key]
		s.sizeUsed -= entrySize(item.key, entry.Value)
		s.lru.remove(item.key)
		delete(s.data, item.key)
		delete(s.heapItems, item.key)
	}
}

// delete removes a key from all structures.
// Must be called with s.mu held.
func (s *Store) delete(key string) {
	entry, ok := s.data[key]
	if !ok {
		return
	}
	s.sizeUsed -= entrySize(key, entry.Value)
	s.tombstone(key)
	s.lru.remove(key)
	delete(s.data, key)
}

// tombstone marks the existing heap entry for a key as deleted (if any).
// Must be called with s.mu held.
func (s *Store) tombstone(key string) {
	if item, ok := s.heapItems[key]; ok {
		item.deleted = true
		delete(s.heapItems, key)
	}
}

func entrySize(key, value string) int64 {
	return int64(len(key) + len(value))
}
