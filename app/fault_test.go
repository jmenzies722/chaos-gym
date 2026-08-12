package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestParseFault(t *testing.T) {
	cases := []struct {
		query    string
		wantErr  bool
		fraction float64
		delay    time.Duration
	}{
		{query: "fraction=0.25&delay=2s", fraction: 0.25, delay: 2 * time.Second},
		{query: "fraction=0", fraction: 0, delay: 0}, // clears, delay not required
		{query: "fraction=1&delay=500ms", fraction: 1, delay: 500 * time.Millisecond},
		{query: "fraction=1.5&delay=1s", wantErr: true},     // out of range
		{query: "fraction=-0.1&delay=1s", wantErr: true},    // negative
		{query: "fraction=abc&delay=1s", wantErr: true},     // not a number
		{query: "fraction=0.5", wantErr: true},              // delay required when injecting
		{query: "fraction=0.5&delay=potato", wantErr: true}, // unparseable duration
	}

	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "/chaos/latency?"+c.query, nil)
		got, err := parseFault(r)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected an error, got %+v", c.query, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.query, err)
			continue
		}
		if got.Fraction != c.fraction || got.Delay != c.delay {
			t.Errorf("%s: got %v/%v, want %v/%v", c.query, got.Fraction, got.Delay, c.fraction, c.delay)
		}
	}
}

// TestFaultStoreConcurrentAccess is here for `go test -race` rather than for
// its assertions: it reproduces the real shape of the bug, one goroutine
// writing the fault while many read it on the request path.
func TestFaultStoreConcurrentAccess(t *testing.T) {
	s := newFaultStore()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.get()
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.set(latencyFault{Fraction: float64(n) / 10, Delay: time.Second})
		}(i)
	}
	wg.Wait()
}

func TestWorkAppliesInjectedLatency(t *testing.T) {
	faults := newFaultStore()
	// Fraction 1 makes this deterministic — every request takes the hit, so the
	// test does not depend on a random draw going the right way.
	faults.set(latencyFault{Fraction: 1, Delay: 80 * time.Millisecond})

	h := handleWork(10*time.Millisecond, faults)
	r := httptest.NewRequest(http.MethodGet, "/work", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	h(w, r)
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("took %s, expected at least the 80ms injected delay", elapsed)
	}
}

func TestWorkIsFastWithNoFault(t *testing.T) {
	h := handleWork(10*time.Millisecond, newFaultStore())
	r := httptest.NewRequest(http.MethodGet, "/work", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	h(w, r)

	if elapsed := time.Since(start); elapsed > 60*time.Millisecond {
		t.Errorf("took %s with no fault set, expected roughly the 10ms base", elapsed)
	}
}
