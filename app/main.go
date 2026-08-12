package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

	tp, err := newTracerProvider(context.Background())
	if err != nil {
		log.Fatalf("tracing setup: %v", err)
	}

	faults := newFaultStore()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /work", handleWork(workDuration, faults))

	// Fault injection is off unless explicitly switched on. An endpoint that
	// makes your own service slow is a denial-of-service switch, and it should
	// never be reachable by default just because the binary supports it. The
	// manifest enables it for this cluster deliberately.
	if envOr("CHAOS_API_ENABLED", "") == "true" {
		mux.HandleFunc("/chaos/latency", handleChaosLatency(faults))
		log.Print("chaos API enabled at /chaos/latency")
	}

	// One span per inbound request, created before routing happens. The name
	// formatter uses the path because r.Pattern is not populated yet out here;
	// that is safe only because every path this service serves is fixed. A
	// path with an ID in it would blow up span cardinality.
	handler := otelhttp.NewHandler(mux, "",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)

	addr := ":" + envOr("PORT", "8080")
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
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

	// Server first, tracing second. The batcher is still holding spans from
	// the requests that just finished draining; shutting it down first would
	// throw away telemetry for exactly the requests a pod kill interrupts —
	// the ones this project exists to look at.
	if err := tp.Shutdown(shutdownCtx); err != nil {
		log.Printf("flushing traces: %v", err)
	}
	log.Print("drained cleanly")
}

// handleHealthz is what the Kubernetes liveness probe calls. It answers for
// this process only — it must never check a database or a downstream service,
// or one slow dependency causes k8s to restart every healthy pod behind it.
func handleHealthz( w http.ResponseWriter,r *http.Request )   {
	w.WriteHeader(http.StatusOK)
}

// handleWork simulates a unit of real work. The delay exists so there is
// latency worth measuring once OpenTelemetry goes in, and so a pod kill has a
// realistic chance of landing mid-request.
func handleWork(d time.Duration, faults *faultStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		delay := d
		if f := faults.get(); f.Fraction > 0 && rand.Float64() < f.Fraction {
			delay += f.Delay
		}

		select {
		case <-time.After(delay):
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
