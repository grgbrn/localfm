package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"bitbucket.org/grgbrn/localfm/pkg/query"
)

func renderTemplate(w http.ResponseWriter, tmpl string) {
	templatePath := "./ui/html/" + tmpl + ".html"
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}

	err = t.Execute(w, nil)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
	}
}

func renderJSON(w http.ResponseWriter, status int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	w.Write([]byte(response))
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, "./recent", http.StatusTemporaryRedirect)
}

// primary html templates
func recentPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "recent")
}

func monthlyPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "monthly")
}

func artistsPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "artists")
}

// json data handlers
// XXX rename these handlers to match the new query names
func (app *application) artistsData(w http.ResponseWriter, r *http.Request) {

	// XXX need correct parameters here
	start := "2019-06-01"
	end := "2019-07-01"
	lim := 20

	artists, err := query.TopArtists(app.db, start, end, lim)
	if err != nil {
		// XXX not sure i want to expose the error string here
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderJSON(w, http.StatusOK, artists)
}

func (app *application) monthlyArtistData(w http.ResponseWriter, r *http.Request) {
	// XXX need correct parameters here
	start := "2019-06-01"
	end := "2019-07-01"
	lim := 20

	artists, err := query.TopNewArtists(app.db, start, end, lim)
	if err != nil {
		// XXX not sure i want to expose the error string here
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderJSON(w, http.StatusOK, artists)
}

func (app *application) monthlyTrackData(w http.ResponseWriter, r *http.Request) {

	// XXX need correct parameters here
	start := "2019-06-01"
	end := "2019-07-01"
	lim := 20

	artists, err := query.TopTracks(app.db, start, end, lim)
	if err != nil {
		// XXX not sure i want to expose the error string here
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderJSON(w, http.StatusOK, artists)
}

func (app *application) listeningClockData(w http.ResponseWriter, r *http.Request) {

	clock, err := query.ListeningClock(app.db, 4, 2019) // XXX need correct params
	if err != nil {
		// XXX not sure i want to expose the error string here
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderJSON(w, http.StatusOK, clock)
}
