package http

import (
	"encoding/json"
	"net/http"
	"time"

	"cache/internal/core/service"
)

type Handler struct {
	svc *service.CacheService
}

func NewHandler(svc *service.CacheService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/cache", h.handleCache)
}

// PUT /cache?key=foo[&ttl=30s]  body: {"value":"bar"}
// GET /cache?key=foo
// POST /cache?key=foo  (remove)
func (h *Handler) handleCache(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "missing query param: key", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		if raw := r.URL.Query().Get("ttl"); raw != "" {
			ttl, err := time.ParseDuration(raw)
			if err != nil || ttl <= 0 {
				http.Error(w, "invalid ttl: use Go duration format e.g. 30s, 5m", http.StatusBadRequest)
				return
			}
			h.svc.SetWithTTL(key, body.Value, ttl)
		} else {
			h.svc.Set(key, body.Value)
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodGet:
		value, ok := h.svc.Get(key)
		if !ok {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"value": value})

	case http.MethodPost:
		h.svc.Remove(key)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
