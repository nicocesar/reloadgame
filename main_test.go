package main

import (
	"math"
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
