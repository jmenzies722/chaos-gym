package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// shutdownGrace is how long in-flight requests get to finish after SIGTERM.
// It must stay below the pod's terminationGracePeriodSeconds (30s by default)
// or Kubernetes sends SIGKILL while the server is still draining.
const shutdownGrace = 10 * time.Second

func main() {
	workDuration, err := time.ParseDuration(envOr("WORK_DURATION", "100ms"))
	if err != nil {
		log.Fatalf("invalid WORK_DURATION: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /work", handleWork(workDuration))

	addr := ":" + envOr("PORT", "8080")
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Kubernetes signals a pod kill with SIGTERM. Catching it is what turns an
	// abrupt connection reset into a clean drain.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		log.Printf("listening on %s (work duration %s)", addr, workDuration)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server stopped: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("shutdown signal received, draining")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	// Shutdown stops accepting new connections, then waits for active requests
	// to finish. It does not cancel them.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("drain failed, forcing exit: %v", err)
	}
	log.Print("drained cleanly")
}

// handleHealthz is what the Kubernetes liveness probe calls. It answers for
// this process only — it must never check a database or a downstream service,
// or one slow dependency causes k8s to restart every healthy pod behind it.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleWork simulates a unit of real work. The delay exists so there is
// latency worth measuring once OpenTelemetry goes in, and so a pod kill has a
// realistic chance of landing mid-request.
func handleWork(d time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(d):
		case <-r.Context().Done():
			// Client hung up. Nothing to write to.
			return
		}

		host, err := os.Hostname()
		if err != nil {
			http.Error(w, "hostname unavailable", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"pod": host}); err != nil {
			log.Printf("encoding response: %v", err)
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
