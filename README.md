# go-cache-server-naive

A study-case in-memory key-value cache server written in Go.

## HTTP API

All operations go through a single endpoint: `GET|PUT|POST /cache?key=<key>`.

### Set a value (no TTL)

```
PUT /cache?key=<key>
Content-Type: application/json

{"value": "<value>"}
```

Response: `204 No Content`

### Set a value with TTL

```
PUT /cache?key=<key>&ttl=<duration>
Content-Type: application/json

{"value": "<value>"}
```

`ttl` uses Go duration format: `30s`, `5m`, `1h30m`, etc.

Response: `204 No Content`

### Get a value

```
GET /cache?key=<key>
```

Response `200 OK`:
```json
{"value": "<value>"}
```

Response `404 Not Found` if the key does not exist or has expired.

### Delete a value

```
POST /cache?key=<key>
```

Response: `204 No Content`

## Design

### Architecture

The service follows a hexagonal (ports & adapters) layout:

```
cmd/main.go
internal/
  core/
    domain/       — CacheEntry, EvictionPolicy
    port/         — Cache interface
    service/      — CacheService (thin orchestration layer)
  adapters/
    inbound/http/ — HTTP handler, route registration
    outbound/in_memory/ — Store implementation
```

### In-memory store

The store (`internal/adapters/outbound/in_memory/store.go`) combines three data structures behind a single `sync.RWMutex`:

| Structure | Purpose |
|---|---|
| `map[string]CacheEntry` | O(1) key lookup; holds value and optional expiry timestamp |
| `lruList` (doubly-linked list + map) | Tracks access order for LRU eviction |
| `expiryHeap` (min-heap) | Tracks TTL expiry order for efficient time-based eviction |

### TTL eviction

- A background goroutine runs every **1 second** and pops all entries from the min-heap whose `expiresAt` is in the past.
- On `Get`, expiry is also checked inline (lazy expiration) so a key is never returned after its TTL.
- Overwriting a key tombstones its old heap entry (marks `deleted = true`) rather than doing an O(n) heap remove, keeping writes O(log n).

### LRU eviction

- Every `Set`/`SetWithTTL` pushes the key to the **front** of the LRU list.
- Every `Get` moves the key to the **front** (touch).
- When a write causes `sizeUsed > sizeLimit`, entries are popped from the **back** (least recently used) until the limit is satisfied.
- Default size limit: **4 GB** (measured as `len(key) + len(value)` bytes).

### Concurrency

All reads and writes acquire `sync.RWMutex`. `Get` acquires a write lock (not read lock) because it mutates the LRU list on touch.
