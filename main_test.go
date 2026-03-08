package main

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/philippgille/gokv/syncmap"
)

func TestNavCheckHandler(t *testing.T) {
	tests := []struct {
		name        string
		navType     string
		session     *SessionData
		wantMessage string
	}{
		{
			name:        "reload with no session returns reload message",
			navType:     "reload",
			session:     nil,
			wantMessage: "Reload this page",
		},
		{
			name:    "reload with session not won returns ending 1",
			navType: "reload",
			session: &SessionData{
				Visits:    1,
				LastVisit: time.Now(),
			},
			wantMessage: "Congratulations you won the game (Ending 1)!",
		},
		{
			name:    "reload with session already won returns ending 2",
			navType: "reload",
			session: &SessionData{
				HasWon:    true,
				Visits:    2,
				LastVisit: time.Now(),
			},
			wantMessage: "You lose! (Ending 2)",
		},
		{
			name:    "reload with session already won multiple times",
			navType: "reload",
			session: &SessionData{
				HasWon:       true,
				Ending2Count: 1,
				Visits:       3,
				LastVisit:    time.Now(),
			},
			wantMessage: "You lose 2 times! (Ending 2)",
		},
		{
			name:    "navigate with session resets to reload message",
			navType: "navigate",
			session: &SessionData{
				HasWon:    true,
				Visits:    5,
				LastVisit: time.Now(),
			},
			wantMessage: "Reload this page",
		},
		{
			name:        "navigate with no session returns reload message",
			navType:     "navigate",
			session:     nil,
			wantMessage: "Reload this page",
		},
		{
			name:    "empty type treated as direct access",
			navType: "",
			session: &SessionData{
				HasWon:    true,
				Visits:    2,
				LastVisit: time.Now(),
			},
			wantMessage: "Reload this page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics = syncmap.NewStore(syncmap.DefaultOptions)

			body, _ := json.Marshal(navCheckRequest{Type: tt.navType})
			req := httptest.NewRequest(http.MethodPost, "/nav-check", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.session != nil {
				// Set session cookie on request
				rec := httptest.NewRecorder()
				saveSession(rec, tt.session)
				for _, c := range rec.Result().Cookies() {
					req.AddCookie(c)
				}
			}

			rec := httptest.NewRecorder()
			navCheckHandler(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}

			var resp navCheckResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", resp.Message, tt.wantMessage)
			}
		})
	}
}

func TestNavCheckHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/nav-check", nil)
	rec := httptest.NewRecorder()
	navCheckHandler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestHandlerServesHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}
	// Should not set a session cookie
	if len(rec.Result().Cookies()) != 0 {
		t.Error("handler should not set cookies")
	}
}

func TestHandlerReturns404ForOtherPaths(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestRecordEnding(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	tests := []struct {
		name      string
		ending    int
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid ending 1",
			ending:    1,
			wantError: false,
		},
		{
			name:      "valid ending 2",
			ending:    2,
			wantError: false,
		},
		{
			name:      "invalid ending 0",
			ending:    0,
			wantError: true,
			errorMsg:  "invalid ending: 0",
		},
		{
			name:      "invalid ending -42",
			ending:    -42,
			wantError: true,
			errorMsg:  "invalid ending: -42",
		},
		{
			name:      "invalid ending 3",
			ending:    3,
			wantError: true,
			errorMsg:  "invalid ending: 3",
		},
		{
			name:      "invalid ending MAXINT",
			ending:    math.MaxInt,
			wantError: true,
			errorMsg:  "invalid ending: 9223372036854775807",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := recordEnding(tt.ending)
			if tt.wantError {
				if err == nil {
					t.Errorf("recordEnding(%d) expected error, got nil", tt.ending)
					return
				}
				if err.Error() != tt.errorMsg {
					t.Errorf("recordEnding(%d) error = %q, want %q", tt.ending, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("recordEnding(%d) unexpected error: %v", tt.ending, err)
				}
			}
		})
	}
}

func TestRecordEndingMetricsStored(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	if err := recordEnding(1); err != nil {
		t.Fatalf("recordEnding(1) unexpected error: %v", err)
	}

	if err := recordEnding(1); err != nil {
		t.Fatalf("recordEnding(1) second call unexpected error: %v", err)
	}

	if err := recordEnding(2); err != nil {
		t.Fatalf("recordEnding(2) unexpected error: %v", err)
	}

	var timestamps1 []time.Time
	found, err := metrics.Get("Ending1", &timestamps1)
	if err != nil {
		t.Fatalf("metrics.Get error: %v", err)
	}
	if !found {
		t.Fatal("Expected to find Ending1 in metrics")
	}
	if len(timestamps1) != 2 {
		t.Errorf("Expected 2 timestamps for Ending1, got %d", len(timestamps1))
	}

	var timestamps2 []time.Time
	found, err = metrics.Get("Ending2", &timestamps2)
	if err != nil {
		t.Fatalf("metrics.Get error: %v", err)
	}
	if !found {
		t.Fatal("Expected to find Ending2 in metrics")
	}
	if len(timestamps2) != 1 {
		t.Errorf("Expected 1 timestamp for Ending2, got %d", len(timestamps2))
	}
}
