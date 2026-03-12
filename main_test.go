package main

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
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
			name:      "valid ending 3",
			ending:    3,
			wantError: false,
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

func TestMetricsHandlerAuth(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	t.Run("no token configured allows access", func(t *testing.T) {
		metricsAuthToken = ""
		req := httptest.NewRequest(http.MethodGet, "/metrics/endings", nil)
		rr := httptest.NewRecorder()
		metricsHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("token configured, no Authorization header returns 401", func(t *testing.T) {
		metricsAuthToken = "secret"
		defer func() { metricsAuthToken = "" }()
		req := httptest.NewRequest(http.MethodGet, "/metrics/endings", nil)
		rr := httptest.NewRecorder()
		metricsHandler(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("token configured, wrong token returns 401", func(t *testing.T) {
		metricsAuthToken = "secret"
		defer func() { metricsAuthToken = "" }()
		req := httptest.NewRequest(http.MethodGet, "/metrics/endings", nil)
		req.Header.Set("Authorization", "Bearer wrongtoken")
		rr := httptest.NewRecorder()
		metricsHandler(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("token configured, non-Bearer scheme returns 401", func(t *testing.T) {
		metricsAuthToken = "secret"
		defer func() { metricsAuthToken = "" }()
		req := httptest.NewRequest(http.MethodGet, "/metrics/endings", nil)
		req.Header.Set("Authorization", "Basic secret")
		rr := httptest.NewRecorder()
		metricsHandler(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("token configured, correct token returns 200", func(t *testing.T) {
		metricsAuthToken = "secret"
		defer func() { metricsAuthToken = "" }()
		req := httptest.NewRequest(http.MethodGet, "/metrics/endings", nil)
		req.Header.Set("Authorization", "Bearer secret")
		rr := httptest.NewRecorder()
		metricsHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
func TestNavCheckShowClickMeWhenBothEndingsDone(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	// Session with both ending 1 and ending 2 completed
	session := &SessionData{
		HasWon:       false, // reset by navigate
		Ending1Count: 1,
		Ending2Count: 1,
		Visits:       3,
		LastVisit:    time.Now(),
	}

	body, _ := json.Marshal(navCheckRequest{Type: "navigate"})
	req := httptest.NewRequest(http.MethodPost, "/nav-check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	saveSession(rec, session)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	rec = httptest.NewRecorder()
	navCheckHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp navCheckResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Message != "Reload this page" {
		t.Errorf("message = %q, want %q", resp.Message, "Reload this page")
	}
	if !resp.ShowClickMe {
		t.Error("expected show_click_me to be true when both endings are done")
	}
}

func TestNavCheckNoClickMeWithoutBothEndings(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	tests := []struct {
		name    string
		session *SessionData
	}{
		{
			name: "only ending 1 done",
			session: &SessionData{
				Ending1Count: 1,
				Ending2Count: 0,
				Visits:       2,
				LastVisit:    time.Now(),
			},
		},
		{
			name: "only ending 2 done",
			session: &SessionData{
				Ending1Count: 0,
				Ending2Count: 1,
				HasWon:       true,
				Visits:       2,
				LastVisit:    time.Now(),
			},
		},
		{
			name:    "no session",
			session: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(navCheckRequest{Type: "navigate"})
			req := httptest.NewRequest(http.MethodPost, "/nav-check", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.session != nil {
				rec := httptest.NewRecorder()
				saveSession(rec, tt.session)
				for _, c := range rec.Result().Cookies() {
					req.AddCookie(c)
				}
			}

			rec := httptest.NewRecorder()
			navCheckHandler(rec, req)

			var resp navCheckResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.ShowClickMe {
				t.Error("expected show_click_me to be false")
			}
		})
	}
}

func TestCongratulationsHandlerEnding3(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	// Session with both endings done
	session := &SessionData{
		Ending1Count: 1,
		Ending2Count: 1,
		Visits:       3,
		LastVisit:    time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/congratulations", nil)
	rec := httptest.NewRecorder()
	saveSession(rec, session)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	rec = httptest.NewRecorder()
	congratulationsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Congratulations! (Ending 3)") {
		t.Error("expected congratulations page to contain 'Congratulations! (Ending 3)'")
	}

	// Verify session was updated - should have ending3 count and be reset (HasWon=false)
	var updatedSession SessionData
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName {
			req2 := httptest.NewRequest(http.MethodGet, "/", nil)
			req2.AddCookie(c)
			s := getSession(req2)
			if s != nil {
				updatedSession = *s
			}
		}
	}

	if updatedSession.Ending3Count != 1 {
		t.Errorf("expected Ending3Count=1, got %d", updatedSession.Ending3Count)
	}
	if updatedSession.HasWon {
		t.Error("expected HasWon to be false after congratulations (reset for next reload)")
	}
	if updatedSession.Ending1Count != 1 {
		t.Errorf("expected Ending1Count preserved at 1, got %d", updatedSession.Ending1Count)
	}
	if updatedSession.Ending2Count != 1 {
		t.Errorf("expected Ending2Count preserved at 1, got %d", updatedSession.Ending2Count)
	}
}

func TestCongratulationsHandlerRedirectsWithoutEndings(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	tests := []struct {
		name    string
		session *SessionData
	}{
		{
			name:    "no session",
			session: nil,
		},
		{
			name: "only ending 1",
			session: &SessionData{
				Ending1Count: 1,
				Ending2Count: 0,
			},
		},
		{
			name: "only ending 2",
			session: &SessionData{
				Ending1Count: 0,
				Ending2Count: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/congratulations", nil)

			if tt.session != nil {
				rec := httptest.NewRecorder()
				saveSession(rec, tt.session)
				for _, c := range rec.Result().Cookies() {
					req.AddCookie(c)
				}
			}

			rec := httptest.NewRecorder()
			congratulationsHandler(rec, req)

			if rec.Code != http.StatusFound {
				t.Errorf("expected redirect (302), got %d", rec.Code)
			}
			loc := rec.Header().Get("Location")
			if loc != "/" {
				t.Errorf("expected redirect to /, got %q", loc)
			}
		})
	}
}

func TestCongratulationsHandlerMultipleEnding3(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	session := &SessionData{
		Ending1Count: 1,
		Ending2Count: 1,
		Visits:       3,
		LastVisit:    time.Now(),
	}

	// Hit congratulations twice
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/congratulations", nil)
		rec := httptest.NewRecorder()
		saveSession(rec, session)
		for _, c := range rec.Result().Cookies() {
			req.AddCookie(c)
		}

		rec = httptest.NewRecorder()
		congratulationsHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("iteration %d: expected status 200, got %d", i, rec.Code)
		}

		// Update session from response cookie for next iteration
		for _, c := range rec.Result().Cookies() {
			if c.Name == cookieName {
				req2 := httptest.NewRequest(http.MethodGet, "/", nil)
				req2.AddCookie(c)
				session = getSession(req2)
			}
		}
	}

	if session.Ending3Count != 2 {
		t.Errorf("expected Ending3Count=2 after two visits, got %d", session.Ending3Count)
	}
}

func TestMetricsHandlerIncludesEnding3(t *testing.T) {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	if err := recordEnding(1); err != nil {
		t.Fatalf("recordEnding(1) unexpected error: %v", err)
	}
	if err := recordEnding(2); err != nil {
		t.Fatalf("recordEnding(2) unexpected error: %v", err)
	}
	if err := recordEnding(3); err != nil {
		t.Fatalf("recordEnding(3) unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics/endings", nil)
	rr := httptest.NewRecorder()
	metricsHandler(rr, req)

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

	if response.Total != 3 {
		t.Errorf("Expected total to be 3, got %d", response.Total)
	}

	// Check ending 3 is present
	foundEnding3 := false
	for _, d := range response.Data {
		if d.Ending == 3 {
			foundEnding3 = true
			break
		}
	}
	if !foundEnding3 {
		t.Error("Expected to find ending 3 in metrics data")
	}
}
