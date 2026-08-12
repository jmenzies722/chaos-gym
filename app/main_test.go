package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthzReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	handleHealthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWorkReturnsPodName(t *testing.T) {
	rec := httptest.NewRecorder()
	handleWork(time.Millisecond)(rec, httptest.NewRequest(http.MethodGet, "/work", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["pod"] == "" {
		t.Error("pod field is empty; the chaos scheduler relies on it to show which pod served a request")
	}
}

// The mux is what rejects a wrong method, not the handler, so this exercises
// the routing rather than handleWork directly.
func TestWorkRejectsPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /work", handleWork(time.Millisecond))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/work", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
