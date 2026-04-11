package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/yourusername/helix/internal/afdb"
	"github.com/yourusername/helix/internal/api"
	"github.com/yourusername/helix/internal/cache"
	"github.com/yourusername/helix/internal/esm"
	"github.com/yourusername/helix/internal/queue"
	"github.com/yourusername/helix/internal/router"
	"github.com/yourusername/helix/internal/worker"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	esmClient := esm.NewClient()
	afdbClient := afdb.NewClient()
	foldCache := cache.NewCache(redisAddr)
	jobQueue := queue.NewQueue(redisAddr)
	foldRouter := router.NewRouter(foldCache, afdbClient, esmClient)
	handler := api.NewHandler(foldRouter, jobQueue)
	w := worker.NewWorker(jobQueue, esmClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(130 * time.Second))

	r.Get("/health", handler.Health)
	r.Post("/fold", handler.Fold)
	r.Post("/fold/batch", handler.BatchFold)
	r.Post("/fold/async", handler.EnqueueFold)
	r.Get("/fold/jobs/{id}", handler.GetJob)
	r.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Printf("helix listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("helix stopped")
}
