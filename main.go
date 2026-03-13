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
	"unicode"

	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
)

type SessionData struct {
	HasWon       bool      `json:"has_won"`
	Ending1Count int       `json:"ending1_count"`
	Ending2Count int       `json:"ending2_count"`
	Ending3Count int       `json:"ending3_count"`
	Ending4Count int       `json:"ending4_count"`
	Visits       int       `json:"visits"`
	LastVisit    time.Time `json:"last_visit"`
}

type navCheckRequest struct {
	Type string `json:"type"`
}

type navCheckResponse struct {
	Message      string `json:"message"`
	ShowClickMe  bool   `json:"show_click_me,omitempty"`
	ShowNameForm bool   `json:"show_name_form,omitempty"`
}

var (
	cookieName = "reloadgame_session"
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
		.container {
			text-align: center;
			opacity: 0;
			transition: opacity 0.3s ease-in;
		}
		.container.visible {
			opacity: 1;
		}
		h1 {
			margin-bottom: 0.2em;
		}
		.click-me {
			font-size: 0.6em;
			color: #888;
			cursor: pointer;
			text-decoration: none;
		}
		.click-me:hover {
			color: #555;
			text-decoration: underline;
		}
		.name-form {
			margin-top: 1.5em;
		}
		.name-form label {
			display: block;
			margin-bottom: 0.4em;
			color: #555;
		}
		.name-form input[type="text"] {
			padding: 0.4em 0.6em;
			font-size: 1em;
			border: 1px solid #ccc;
			border-radius: 4px;
			width: 220px;
		}
		.name-form button {
			display: block;
			margin: 0.6em auto 0;
			padding: 0.4em 1.2em;
			font-size: 1em;
			cursor: pointer;
		}
		.name-error {
			color: #c00;
			margin-top: 0.4em;
			font-size: 0.9em;
		}
	</style>
</head>
<body>
	<div class="container" id="content">
		<h1 id="msg"></h1>
		<a id="click-me" class="click-me" href="/congratulations" style="display:none">(or click on me)</a>
		<div class="name-form" id="name-form-section" style="display:none">
			<form id="name-form">
				<label for="player-name">type your name:</label>
				<input type="text" id="player-name" name="name" autocomplete="off" />
				<button type="submit">Go</button>
			</form>
			<div class="name-error" id="name-error" style="display:none"></div>
		</div>
	</div>
	<noscript><h1 style="opacity:1">JavaScript is required to play this game.</h1></noscript>
	<script>
		(function() {
			var navType = 'navigate';
			try {
				var entry = performance.getEntriesByType('navigation')[0];
				if (entry && entry.type) {
					navType = entry.type;
				}
			} catch(e) {}
			fetch('/nav-check', {
				method: 'POST',
				headers: {'Content-Type': 'application/json'},
				body: JSON.stringify({type: navType})
			})
			.then(function(r) { return r.json(); })
			.then(function(data) {
				var el = document.getElementById('msg');
				el.textContent = data.message;
				if (data.show_click_me) {
					document.getElementById('click-me').style.display = '';
				}
				if (data.show_name_form) {
					document.getElementById('name-form-section').style.display = '';
				}
				document.getElementById('content').classList.add('visible');
			});

			var form = document.getElementById('name-form');
			form.addEventListener('submit', function(e) {
				e.preventDefault();
				var name = document.getElementById('player-name').value;
				var errEl = document.getElementById('name-error');
				errEl.style.display = 'none';
				fetch('/submit-name', {
					method: 'POST',
					headers: {'Content-Type': 'application/json'},
					body: JSON.stringify({name: name})
				})
				.then(function(r) { return r.json(); })
				.then(function(data) {
					if (data.ok) {
						window.location.href = '/congratulations-4';
					} else {
						errEl.textContent = data.error || 'Invalid name';
						errEl.style.display = '';
					}
				});
			});
		})();
	</script>
</body>
</html>`

const congratulationsTemplate = `<!DOCTYPE html>
<html>
<head>
	<title>Reload Game - Congratulations!</title>
	<style>
		body {
			display: flex;
			justify-content: center;
			align-items: center;
			height: 100vh;
			margin: 0;
			font-family: Arial, sans-serif;
		}
		.container {
			text-align: center;
			opacity: 0;
			transition: opacity 0.3s ease-in;
		}
		.container.visible {
			opacity: 1;
		}
		h1 {
			margin-bottom: 0.2em;
		}
		.name-form {
			margin-top: 1.5em;
		}
		.name-form label {
			display: block;
			margin-bottom: 0.4em;
			color: #555;
		}
		.name-form input[type="text"] {
			padding: 0.4em 0.6em;
			font-size: 1em;
			border: 1px solid #ccc;
			border-radius: 4px;
			width: 220px;
		}
		.name-form button {
			display: block;
			margin: 0.6em auto 0;
			padding: 0.4em 1.2em;
			font-size: 1em;
			cursor: pointer;
		}
		.name-error {
			color: #c00;
			margin-top: 0.4em;
			font-size: 0.9em;
		}
	</style>
</head>
<body>
	<div class="container" id="content">
		<h1>Congratulations! (Ending 3)</h1>
		{{if .ShowNameForm}}
		<div class="name-form">
			<form id="name-form">
				<label for="player-name">type your name:</label>
				<input type="text" id="player-name" name="name" autocomplete="off" />
				<button type="submit">Go</button>
			</form>
			<div class="name-error" id="name-error" style="display:none"></div>
		</div>
		{{end}}
	</div>
	<script>
		(function() {
			var navType = 'navigate';
			try {
				var entry = performance.getEntriesByType('navigation')[0];
				if (entry && entry.type) {
					navType = entry.type;
				}
			} catch(e) {}
			if (navType === 'reload') {
				window.location.replace('/');
				return;
			}
			var el = document.getElementById('content');
			el.classList.add('visible');

			var form = document.getElementById('name-form');
			if (form) {
				form.addEventListener('submit', function(e) {
					e.preventDefault();
					var name = document.getElementById('player-name').value;
					var errEl = document.getElementById('name-error');
					errEl.style.display = 'none';
					fetch('/submit-name', {
						method: 'POST',
						headers: {'Content-Type': 'application/json'},
						body: JSON.stringify({name: name})
					})
					.then(function(r) { return r.json(); })
					.then(function(data) {
						if (data.ok) {
							window.location.href = '/congratulations-4';
						} else {
							errEl.textContent = data.error || 'Invalid name';
							errEl.style.display = '';
						}
					});
				});
			}
		})();
	</script>
</body>
</html>`

const congratulations4Template = `<!DOCTYPE html>
<html>
<head>
	<title>Reload Game - Congratulations! (Ending 4)</title>
	<style>
		body {
			display: flex;
			justify-content: center;
			align-items: center;
			height: 100vh;
			margin: 0;
			font-family: Arial, sans-serif;
		}
		.container {
			text-align: center;
			opacity: 0;
			transition: opacity 0.3s ease-in;
		}
		.container.visible {
			opacity: 1;
		}
		h1 {
			margin-bottom: 0.2em;
		}
	</style>
</head>
<body>
	<div class="container" id="content">
		<h1>Congratulations! (Ending 4)</h1>
	</div>
	<script>
		(function() {
			var navType = 'navigate';
			try {
				var entry = performance.getEntriesByType('navigation')[0];
				if (entry && entry.type) {
					navType = entry.type;
				}
			} catch(e) {}
			if (navType === 'reload') {
				window.location.replace('/');
				return;
			}
			var el = document.getElementById('content');
			el.classList.add('visible');
		})();
	</script>
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

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Surrogate-Control", "no-store")

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    encoded,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	})
}

func recordEnding(ending int) error {
	if ending < 1 || ending > 4 {
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
	metricsMu.Lock()
	defer metricsMu.Unlock()

	type endingRecord struct {
		Timestamp time.Time `json:"timestamp"`
		Ending    int       `json:"ending"`
	}

	records := []endingRecord{}

	for _, ending := range []int{1, 2, 3, 4} {
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

// isValidName returns true if name is a valid player name.
// Rules (relaxed to include more names rather than fewer):
//   - Must not be empty
//   - Each rune must be a letter (any script), digit, space, hyphen, apostrophe, or quote mark
//
// This allows names like: Nicolás, 张, O'Brian, Jean-Pierre, etc.
func isValidName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case ' ', '-', '\'', '\u2019', '"', '\u201C', '\u201D', '.':
			// space, hyphen, apostrophe (straight & curly), double-quote (straight & curly), period
			continue
		default:
			return false
		}
	}
	return true
}

type submitNameRequest struct {
	Name string `json:"name"`
}

type submitNameResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func submitNameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req submitNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !isValidName(req.Name) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(submitNameResponse{OK: false, Error: "Name must only contain letters, digits, spaces, hyphens, apostrophes, or quotes."})
		return
	}

	session := getSession(r)
	// Only accessible if user has ending 3 badge
	if session == nil || session.Ending3Count < 1 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(submitNameResponse{OK: false, Error: "You need to reach Ending 3 first."})
		return
	}

	session.Ending4Count++
	session.Visits++
	session.LastVisit = time.Now()
	if err := recordEnding(4); err != nil {
		log.Printf("Error recording ending 4: %v", err)
	}
	saveSession(w, session)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(submitNameResponse{OK: true})
}

func congratulations4Handler(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)

	// Only accessible if user has ending 3 badge
	if session == nil || session.Ending3Count < 1 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, congratulations4Template)
}

var congratulationsTmpl = template.Must(template.New("congratulations").Parse(congratulationsTemplate))

func congratulationsHandler(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)

	// Only accessible if user has both ending 1 and 2
	if session == nil || session.Ending1Count < 1 || session.Ending2Count < 1 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Record ending 3 and update session
	session.Visits++
	session.LastVisit = time.Now()
	session.Ending3Count++
	if err := recordEnding(3); err != nil {
		log.Printf("Error recording ending 3: %v", err)
	}

	// Reset session so reloading goes back to "Reload this page"
	// but preserve the ending counts so "(or click on me)" remains
	resetSession := &SessionData{
		Ending1Count: session.Ending1Count,
		Ending2Count: session.Ending2Count,
		Ending3Count: session.Ending3Count,
		Ending4Count: session.Ending4Count,
		Visits:       session.Visits,
		LastVisit:    session.LastVisit,
	}
	saveSession(w, resetSession)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		ShowNameForm bool
	}{
		ShowNameForm: resetSession.Ending3Count >= 1,
	}
	if err := congratulationsTmpl.Execute(w, data); err != nil {
		log.Printf("Error rendering congratulations template: %v", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pageTemplate)
}

func navCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req navCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	session := getSession(r)

	var message string

	if req.Type != "reload" {
		// Direct access (typed URL, bookmark, link) — reset session
		// Preserve ending counts so "(or click on me)" remains available
		newSession := &SessionData{
			Visits:    1,
			LastVisit: time.Now(),
		}
		if session != nil {
			newSession.Ending1Count = session.Ending1Count
			newSession.Ending2Count = session.Ending2Count
			newSession.Ending3Count = session.Ending3Count
		}
		session = newSession
		saveSession(w, session)
		message = "Reload this page"
	} else if session == nil {
		// First visit via reload (unlikely but handle it)
		session = &SessionData{
			Visits:    1,
			LastVisit: time.Now(),
		}
		saveSession(w, session)
		message = "Reload this page"
	} else if !session.HasWon {
		// Reload with session, not yet won — win!
		session.Visits++
		session.LastVisit = time.Now()
		session.HasWon = true
		session.Ending1Count++
		if err := recordEnding(1); err != nil {
			log.Printf("Error recording ending 1: %v", err)
		}
		saveSession(w, session)
		message = "Congratulations you won the game (Ending 1)!"
	} else {
		// Reload with session, already won — lose
		session.Visits++
		session.LastVisit = time.Now()
		session.Ending2Count++
		if err := recordEnding(2); err != nil {
			log.Printf("Error recording ending 2: %v", err)
		}
		saveSession(w, session)
		if session.Ending2Count > 1 {
			message = "You lose " + strconv.Itoa(session.Ending2Count) + " times! (Ending 2)"
		} else {
			message = "You lose! (Ending 2)"
		}
	}

	resp := navCheckResponse{Message: message}
	// Show "(or click on me)" when user has completed both ending 1 and 2
	// and is on a "Reload this page" screen
	if message == "Reload this page" && session != nil && session.Ending1Count >= 1 && session.Ending2Count >= 1 {
		resp.ShowClickMe = true
	}
	// Show name form on main page when user has ending 3 badge
	if session != nil && session.Ending3Count >= 1 {
		resp.ShowNameForm = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	mux.HandleFunc("/nav-check", navCheckHandler)
	mux.HandleFunc("/congratulations", congratulationsHandler)
	mux.HandleFunc("/congratulations-4", congratulations4Handler)
	mux.HandleFunc("/submit-name", submitNameHandler)
	mux.HandleFunc("/metrics/endings", metricsHandler)

	log.Printf("listening on http://0.0.0.0%s", addr)
	log.Fatal(http.Serve(ln, mux))
}
