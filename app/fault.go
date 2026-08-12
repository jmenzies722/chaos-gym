package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// latencyFault is the currently injected slowness: a fraction of requests get
// an extra Delay. It is immutable — changing the fault replaces the whole value
// rather than editing fields, which is what makes the atomic swap below safe.
type latencyFault struct {
	Fraction float64       `json:"fraction"`
	Delay    time.Duration `json:"-"`
	DelayStr string        `json:"delay"`
}

// faultStore holds the active fault.
//
// Every HTTP handler runs on its own goroutine, so the request path reads this
// value concurrently with the chaos endpoint writing it. A plain struct field
// would be a data race — not a theoretical one; `go test -race` fails on it,
// and in production it shows up as torn reads under load, which is about the
// worst possible bug to debug. Swapping an immutable pointer makes every reader
// see either the old fault or the new one, never a half-updated mix.
type faultStore struct {
	v atomic.Pointer[latencyFault]
}

func newFaultStore() *faultStore {
	s := &faultStore{}
	s.set(latencyFault{})
	return s
}

func (s *faultStore) get() latencyFault  { return *s.v.Load() }
func (s *faultStore) set(f latencyFault) { s.v.Store(&f) }

// handleChaosLatency reads and sets the injected latency at runtime.
//
// Runtime rather than an environment variable on purpose. Changing an env var
// means editing the Deployment, which rolls the pods — and a rollout is a
// *different* failure with a different signature. Injecting at runtime leaves
// the pods untouched, so the only thing that changes is how long requests take.
// That is the failure worth practising: nothing restarts, nothing errors,
// Kubernetes still calls the service healthy, and it is degraded anyway.
//
//	GET  /chaos/latency                          → current fault
//	POST /chaos/latency?fraction=0.25&delay=2s   → slow 25% of requests by 2s
//	POST /chaos/latency?fraction=0               → clear
func handleChaosLatency(faults *faultStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			f, err := parseFault(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			faults.set(f)
			log.Printf("chaos: latency fault set to %.2f of requests +%s", f.Fraction, f.Delay)
		}

		current := faults.get()
		current.DelayStr = current.Delay.String()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(current); err != nil {
			log.Printf("encoding chaos response: %v", err)
		}
	}
}

func parseFault(r *http.Request) (latencyFault, error) {
	q := r.URL.Query()

	var f latencyFault
	if _, err := fmt.Sscanf(q.Get("fraction"), "%f", &f.Fraction); err != nil {
		return f, fmt.Errorf("fraction must be a number between 0 and 1")
	}
	if f.Fraction < 0 || f.Fraction > 1 {
		return f, fmt.Errorf("fraction must be between 0 and 1, got %v", f.Fraction)
	}

	// A zero fraction clears the fault, so the delay is irrelevant and need not
	// be supplied.
	if f.Fraction == 0 {
		return latencyFault{}, nil
	}

	d, err := time.ParseDuration(q.Get("delay"))
	if err != nil {
		return f, fmt.Errorf("delay must be a duration like 2s or 500ms")
	}
	if d < 0 {
		return f, fmt.Errorf("delay cannot be negative")
	}
	f.Delay = d
	return f, nil
}
