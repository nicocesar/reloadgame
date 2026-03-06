package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/philippgille/gokv/syncmap"
)

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

func TestMetricsHandlerOutputFormat(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	if err := recordEnding(1); err != nil {
		t.Fatalf("recordEnding(1) unexpected error: %v", err)
	}
	if err := recordEnding(2); err != nil {
		t.Fatalf("recordEnding(2) unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics/endings", nil)
	rr := httptest.NewRecorder()
	metricsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response struct {
		Total int `json:"total"`
		Data  []struct {
			Timestamp time.Time `json:"timestamp"`
			Ending    int       `json:"ending"`
		} `json:"data"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Total != 2 {
		t.Errorf("Expected total to be 2, got %d", response.Total)
	}
	if len(response.Data) != 2 {
		t.Errorf("Expected data length to be 2, got %d", len(response.Data))
	}
}

func TestMetricsHandlerZeroData(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	req := httptest.NewRequest(http.MethodGet, "/metrics/endings", nil)
	rr := httptest.NewRecorder()
	metricsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response struct {
		Total int `json:"total"`
		Data  []struct {
			Timestamp time.Time `json:"timestamp"`
			Ending    int       `json:"ending"`
		} `json:"data"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Total != 0 {
		t.Errorf("Expected total to be 0, got %d", response.Total)
	}
	if response.Data == nil {
		t.Error("Expected data to be an empty array, got nil")
	}
	if len(response.Data) != 0 {
		t.Errorf("Expected data length to be 0, got %d", len(response.Data))
	}
}
