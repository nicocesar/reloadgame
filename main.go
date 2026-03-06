package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
)

type SessionData struct {
	HasWon       bool      `json:"has_won"`
	Ending1Count int       `json:"ending1_count"`
	Ending2Count int       `json:"ending2_count"`
	Visits       int       `json:"visits"`
	LastVisit    time.Time `json:"last_visit"`
}

var (
	cookieName = "reloadgame_session"
	tmpl       = template.Must(template.New("page").Parse(pageTemplate))
	metrics    gokv.Store
	metricsMu  sync.Mutex
)

const pageTemplate = `<!DOCTYPE html>
<html>
<head>
	<title>Reload Game</title>
	<style>
		body {
			display: flex;
			justify-content: center;
			align-items: center;
			height: 100vh;
			margin: 0;
			font-family: Arial, sans-serif;
		}
		h1 {
			text-align: center;
		}
	</style>
</head>
<body>
	<h1>{{.Message}}</h1>
</body>
</html>`

func getSession(r *http.Request) *SessionData {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}

	decoded, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}

	var session SessionData
	if err := json.Unmarshal(decoded, &session); err != nil {
		return nil
	}

	return &session
}

func saveSession(w http.ResponseWriter, session *SessionData) {
	data, err := json.Marshal(session)
	if err != nil {
		return
	}

	encoded := base64.URLEncoding.EncodeToString(data)

	http.SetCookie(w, &http.Cookie{
		Name:    cookieName,
		Value:   encoded,
		Path:    "/",
		Expires: time.Now().Add(24 * time.Hour),
	})
}

func recordEnding(ending int) error {
	if ending < 1 || ending > 2 {
		return fmt.Errorf("invalid ending: %d", ending)
	}

	metricsMu.Lock()
	defer metricsMu.Unlock()

	key := "Ending" + strconv.Itoa(ending)
	var timestamps []time.Time

	var item []time.Time
	found, err := metrics.Get(key, &item)
	if err != nil {
		return err
	}
	if found {
		timestamps = item
	}
	timestamps = append(timestamps, time.Now())
	metrics.Set(key, timestamps)
	return nil
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/metrics/endings" {
		http.NotFound(w, r)
		return
	}

	metricsMu.Lock()
	defer metricsMu.Unlock()

	type endingRecord struct {
		Timestamp time.Time `json:"timestamp"`
		Ending    int       `json:"ending"`
	}

	records := []endingRecord{}

	for _, ending := range []int{1, 2} {
		key := "Ending" + strconv.Itoa(ending)
		var item []time.Time
		found, err := metrics.Get(key, &item)
		if err != nil {
			continue
		}
		if !found {
			continue
		}
		for _, ts := range item {
			records = append(records, endingRecord{
				Timestamp: ts,
				Ending:    ending,
			})
		}
	}

	response := struct {
		Total int            `json:"total"`
		Data  []endingRecord `json:"data"`
	}{
		Total: len(records),
		Data:  records,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func isDirectAccess(r *http.Request) bool {
	log.Printf("Sec-Fetch-Site: %s, Cache-Control: %s, Referer: %s", r.Header.Get("Sec-Fetch-Site"), r.Header.Get("Cache-Control"), r.Header.Get("Referer"))
	fetchSite := r.Header.Get("Sec-Fetch-Site")
	if fetchSite != "" {
		if fetchSite != "none" {
			return false
		}
		// Both direct navigation and reload send Sec-Fetch-Site: none.
		// Reloads also send Cache-Control: max-age=0 (soft) or no-cache (hard).
		cc := r.Header.Get("Cache-Control")
		if cc == "max-age=0" || cc == "no-cache" {
			return false
		}
		return true
	}
	// Fallback for browsers without Sec-Fetch-Site support
	return r.Header.Get("Referer") == ""
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/metrics/endings" {
		metricsHandler(w, r)
		return
	}

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	session := getSession(r)

	if session == nil {
		session = &SessionData{
			Visits:    1,
			LastVisit: time.Now(),
		}
		saveSession(w, session)
		tmpl.Execute(w, map[string]string{"Message": "Reload this page"})
		return
	}

	session.Visits++
	session.LastVisit = time.Now()

	if session.HasWon {
		session.Ending2Count++
		if err := recordEnding(2); err != nil {
			log.Printf("Error recording ending 2: %v", err)
		}
		saveSession(w, session)
		if session.Ending2Count > 1 {
			tmpl.Execute(w, map[string]string{"Message": "You lose " + strconv.Itoa(session.Ending2Count) + " times! (Ending 2)"})
		} else {
			tmpl.Execute(w, map[string]string{"Message": "You lose! (Ending 2)"})
		}
		return
	}

	session.HasWon = true
	session.Ending1Count++
	if err := recordEnding(1); err != nil {
		log.Printf("Error recording ending 1: %v", err)
	}

	saveSession(w, session)
	tmpl.Execute(w, map[string]string{"Message": "Congratulations you won the game (Ending 1)!"})
}

func main() {
	metrics = syncmap.NewStore(syncmap.DefaultOptions)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("invalid PORT %q: %v", port, err)
	}

	addr := ":" + port

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("can't listen on %s: %v", addr, err)
	}
	defer ln.Close()

	log.Printf("listening on http://0.0.0.0%s", addr)
	log.Fatal(http.Serve(ln, http.HandlerFunc(handler)))
}
