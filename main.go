package main

import (
	"encoding/base64"
	"encoding/json"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

type SessionData struct {
	HasWon       bool          `json:"has_won"`
	Ending1Count int           `json:"ending1_count"`
	Visits       int           `json:"visits"`
	LastVisit    time.Time     `json:"last_visit"`
}

var (
	cookieName = "reloadgame_session"
	tmpl       = template.Must(template.New("page").Parse(pageTemplate))
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

func isDirectAccess(r *http.Request) bool {
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
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	session := getSession(r)
	directAccess := isDirectAccess(r)

	if session == nil || directAccess {
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

	if !session.HasWon {
		session.HasWon = true
		session.Ending1Count++
	}

	saveSession(w, session)
	tmpl.Execute(w, map[string]string{"Message": "Congratulations you won the game (Ending 1)!"})
}

func main() {
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

	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.Serve(ln, http.HandlerFunc(handler)))
}
