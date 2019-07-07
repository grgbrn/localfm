package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"bitbucket.org/grgbrn/localfm/pkg/query"
)

const logStackTraces bool = false

func (app *application) serverError(w http.ResponseWriter, err error) {
	if logStackTraces {
		// from "let's go" ch3.04
		// XXX but this could use some tweaking
		trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
		//app.err.Println(trace)
		app.err.Output(2, trace)
	} else {
		app.err.Println(err.Error())
	}

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// XXX maybe add a clientError also?
// file:///home/greg/Downloads/lets-go/html/03.04-centralized-error-handling.html

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

// extractDateRangeParams translates mode=X&offset=Y parameters
// from the URL query into start/end/lim parameters expected by
// the query package
func extractDateRangeParams(r *http.Request) (query.DateRangeParams, error) {

	var params query.DateRangeParams

	// required param: mode
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		return params, errors.New("missing required parameter: mode")
	}
	params.Mode = mode

	// required param: offset
	offStr := r.URL.Query().Get("offset")
	if offStr == "" {
		return params, errors.New("missing required parameter: offset")
	}
	offset, err := strconv.Atoi(offStr)
	if err != nil {
		return params, errors.New("invalid format for parameter: offset")
	}

	// optional param: tz
	tzStr := r.URL.Query().Get("tz")
	if tzStr != "" {
		loc, err := time.LoadLocation(tzStr)
		if err != nil {
			fmt.Printf("Error loading timezone:%s %v", tzStr, err)
		} else {
			params.TZ = loc
		}
	}
	if params.TZ == nil {
		params.TZ = time.UTC
	}

	// optional param: count
	// XXX client never actually changes the value
	// countStr := r.URL.Query().Get("count")
	params.Limit = 20

	// compute start/end dates from mode & offset
	// XXX refactor this to be unit-testable and not depend on Now()
	now := time.Now()

	if mode == "week" {
		// show week ending today / last 7 days
		tmp := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, params.TZ)
		params.End = tmp.AddDate(0, 0, -offset*7)
		params.Start = params.End.AddDate(0, 0, -7)
	} else if mode == "month" {
		// show month to date (inconsistent with week)
		tmp := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, params.TZ)
		params.Start = tmp.AddDate(0, -offset, 0)
		params.End = params.Start.AddDate(0, 1, 0)
	} else if mode == "year" {
		y := now.Year()
		y -= offset
		params.Start = time.Date(y, time.January, 1, 0, 0, 0, 0, params.TZ)
		params.End = params.Start.AddDate(1, 0, 0)
	} else {
		return params, errors.New("invalid value for parameter: mode")
	}

	fmt.Printf("{%s %s %d}\n", params.StartString(), params.EndString(), params.Limit)
	return params, nil
}

// json data handlers
func (app *application) topArtistsData(w http.ResponseWriter, r *http.Request) {

	type topArtistsResponse struct {
		Mode      string               `json:"mode"`
		StartDate time.Time            `json:"startDate"`
		EndDate   time.Time            `json:"endDate"`
		Artists   []query.ArtistResult `json:"artists"`
	}

	params, err := extractDateRangeParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	artists, err := query.TopArtists(app.db, params)
	if err != nil {
		app.serverError(w, err)
		return
	}

	renderJSON(w, http.StatusOK, topArtistsResponse{
		Mode:      params.Mode,
		StartDate: params.Start,
		EndDate:   params.End,
		Artists:   artists,
	})
}

func (app *application) topNewArtistsData(w http.ResponseWriter, r *http.Request) {

	// xxx duplicate of topArtistsResponse
	type topNewArtistsResponse struct {
		Mode      string               `json:"mode"`
		StartDate time.Time            `json:"startDate"`
		EndDate   time.Time            `json:"endDate"`
		Artists   []query.ArtistResult `json:"artists"`
	}

	params, err := extractDateRangeParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	artists, err := query.TopNewArtists(app.db, params)
	if err != nil {
		app.serverError(w, err)
		return
	}

	renderJSON(w, http.StatusOK, topNewArtistsResponse{
		Mode:      params.Mode,
		StartDate: params.Start,
		EndDate:   params.End,
		Artists:   artists,
	})
}

func (app *application) topTracksData(w http.ResponseWriter, r *http.Request) {

	type topTracksResponse struct {
		Mode      string              `json:"mode"`
		StartDate time.Time           `json:"startDate"`
		EndDate   time.Time           `json:"endDate"`
		Tracks    []query.TrackResult `json:"tracks"`
	}

	params, err := extractDateRangeParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	topTracks, err := query.TopTracks(app.db, params)
	if err != nil {
		app.serverError(w, err)
		return
	}

	renderJSON(w, http.StatusOK, topTracksResponse{
		Mode:      params.Mode,
		StartDate: params.Start,
		EndDate:   params.End,
		Tracks:    topTracks,
	})
}

func (app *application) listeningClockData(w http.ResponseWriter, r *http.Request) {

	type listeningClockResponse struct {
		Mode      string               `json:"mode"`
		StartDate time.Time            `json:"startDate"`
		EndDate   time.Time            `json:"endDate"`
		Clock     *[]query.ClockResult `json:"clock"`
	}

	params, err := extractDateRangeParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clock, err := query.ListeningClock(app.db, params)
	if err != nil {
		app.serverError(w, err)
		return
	}

	renderJSON(w, http.StatusOK, listeningClockResponse{
		Mode:      params.Mode,
		StartDate: params.Start,
		EndDate:   params.End,
		Clock:     clock,
	})
}
