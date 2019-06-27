package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

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

// many data handlers query over a date range
type dateRangeParams struct {
	start string
	end   string
	limit int
}

func extractDateRangeParams(r *http.Request) (dateRangeParams, error) {

	var params dateRangeParams

	// required param: mode
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		return params, errors.New("missing required parameter: mode")
	}

	// required param: offset
	offStr := r.URL.Query().Get("offset")
	if offStr == "" {
		return params, errors.New("missing required parameter: offset")
	}
	offset, err := strconv.Atoi(offStr)
	if err != nil {
		return params, errors.New("invalid format for parameter: offset")
	}

	// optional param: count
	params.limit = 20
	// countStr := r.URL.Query().Get("count")

	// compute start/end dates from mode & offset
	// XXX refactor this to be unit-testable and not depend on Now()
	now := time.Now()

	if mode == "week" {
		// show week ending today / last 7 days
		end := now.AddDate(0, 0, -offset*7)
		start := end.AddDate(0, 0, -7)
		params.start = start.Format("2006-01-02")
		params.end = end.Format("2006-01-02")
	} else if mode == "month" {
		// show month to date (inconsistent with week)
		start := now.AddDate(0, -offset, 0)
		end := start.AddDate(0, 1, 0)
		params.start = fmt.Sprintf("%d-%02d-01", start.Year(), start.Month())
		params.end = fmt.Sprintf("%d-%02d-01", end.Year(), end.Month())
	} else if mode == "year" {
		y := now.Year()
		y -= offset
		params.start = fmt.Sprintf("%d-01-01", y)
		params.end = fmt.Sprintf("%d-01-01", y+1)
	} else {
		return params, errors.New("invalid value for parameter: mode")
	}

	fmt.Println(params)
	return params, nil
}

// json data handlers
func (app *application) topArtistsData(w http.ResponseWriter, r *http.Request) {

	dp, err := extractDateRangeParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	artists, err := query.TopArtists(app.db, dp.start, dp.end, dp.limit)
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
