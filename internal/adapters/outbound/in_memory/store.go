package in_memory

import (
	"container/heap"
	"context"
	"hash/fnv"
	"sync"
	"time"

	"cache/internal/core/domain"
)

const (
	evictionInterval = time.Second
	numShards        = 256
)

type shard struct {
	mu        sync.Mutex
	data      map[string]domain.CacheEntry
	expiry    *expiryHeap
	heapItems map[string]*heapItem
	lru       *lruList
	sizeUsed  int64
	sizeLimit int64
}

func newShard(sizeLimit int64) *shard {
	return &shard{
		data:      make(map[string]domain.CacheEntry),
		expiry:    newExpiryHeap(),
		heapItems: make(map[string]*heapItem),
		lru:       newLRUList(),
		sizeLimit: sizeLimit,
	}
}

type Store struct {
	shards [numShards]*shard
}

func NewStore(sizeLimit int64) *Store {
	s := &Store{}
	perShard := sizeLimit / numShards
	for i := range s.shards {
		s.shards[i] = newShard(perShard)
	}
	return s
}

func shardIndex(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0xFF)
}

func (s *Store) getShard(key string) *shard {
	return s.shards[shardIndex(key)]
}

func (s *Store) Set(key, value string) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.write(key, domain.CacheEntry{Value: value})
}

func (s *Store) SetWithTTL(key, value string, ttl time.Duration) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	expiresAt := time.Now().Add(ttl)
	entry := domain.CacheEntry{Value: value, ExpiresAt: expiresAt}
	sh.write(key, entry)
	item := &heapItem{key: key, expiresAt: expiresAt}
	heap.Push(sh.expiry, item)
	sh.heapItems[key] = item
}

func (s *Store) Get(key string) (string, bool) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	entry, ok := sh.data[key]
	if !ok {
		return "", false
	}
	if entry.IsExpired() {
		sh.delete(key)
		return "", false
	}
	sh.lru.touch(key)
	return entry.Value, true
}

func (s *Store) Remove(key string) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.delete(key)
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
				var wg sync.WaitGroup
				for _, sh := range s.shards {
					wg.Add(1)
					go func(sh *shard) {
						defer wg.Done()
						sh.evictExpired()
					}(sh)
				}
				wg.Wait()
			}
		}
	}()
}

func (sh *shard) write(key string, entry domain.CacheEntry) {
	if old, exists := sh.data[key]; exists {
		sh.sizeUsed -= entrySize(key, old.Value)
		sh.tombstone(key)
	}
	sh.data[key] = entry
	sh.lru.add(key)
	sh.sizeUsed += entrySize(key, entry.Value)
	for sh.sizeLimit > 0 && sh.sizeUsed > sh.sizeLimit {
		sh.evictLRU()
	}
}

func (sh *shard) evictLRU() {
	key := sh.lru.evict()
	if key == "" {
		return
	}
	entry := sh.data[key]
	sh.sizeUsed -= entrySize(key, entry.Value)
	sh.tombstone(key)
	delete(sh.data, key)
}

func (sh *shard) evictExpired() {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	now := time.Now()
	for {
		item := sh.expiry.peek()
		if item == nil {
			break
		}
		if item.deleted {
			heap.Pop(sh.expiry)
			continue
		}
		if item.expiresAt.After(now) {
			break
		}
		heap.Pop(sh.expiry)
		entry := sh.data[item.key]
		sh.sizeUsed -= entrySize(item.key, entry.Value)
		sh.lru.remove(item.key)
		delete(sh.data, item.key)
		delete(sh.heapItems, item.key)
	}
}

func (sh *shard) delete(key string) {
	entry, ok := sh.data[key]
	if !ok {
		return
	}
	sh.sizeUsed -= entrySize(key, entry.Value)
	sh.tombstone(key)
	sh.lru.remove(key)
	delete(sh.data, key)
}

func (sh *shard) tombstone(key string) {
	if item, ok := sh.heapItems[key]; ok {
		item.deleted = true
		delete(sh.heapItems, key)
	}
}

func entrySize(key, value string) int64 {
	return int64(len(key) + len(value))
}
