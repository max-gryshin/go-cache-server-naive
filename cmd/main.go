package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	inboundhttp "cache/internal/adapters/inbound/http"
	"cache/internal/adapters/outbound/in_memory"
	"cache/internal/core/service"
)

const sizeLimit = 4 * 1024 * 1024 * 1024 // 4 GB

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	store := in_memory.NewStore(sizeLimit)
	store.StartEviction(ctx)

	svc := service.NewCacheService(store)
	handler := inboundhttp.NewHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	log.Println("cache server listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
