package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"bitbucket.org/grgbrn/localfm/pkg/query"
)

//
// login pages
//
func (app *Application) loginUser(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		renderTemplate(w, "login", &templateData{})
	} else if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			// XXX should be client error
			// http.StatusBadRequest
			http.Error(w, "Internal Server Error", http.StatusBadRequest)
			return
		}

		// retrieve field values
		email := r.PostForm.Get("email")
		passwd := r.PostForm.Get("password")

		userID, err := authenticateUser(email, passwd)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusBadRequest)
			return
		}
		if userID == -1 {
			renderTemplate(w, "login", &templateData{
				Error: "Email or Password is incorrect",
			})
			return
		}
		app.session.Put(r, "authenticatedUserID", userID)

		// redirect to splash page
		// XXX but we should remember what the user was trying
		// to get to when we intercepted them...
		http.Redirect(w, r, "./recent", http.StatusSeeOther)
	} else {
		http.Error(w, "Internal Server Error", http.StatusBadRequest)
		return
	}
}

func (app *Application) logoutUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Internal Server Error", http.StatusBadRequest)
		return
	}
	app.session.Remove(r, "authenticatedUserID")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

//
// authenticated app pages
//
func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, "./recent", http.StatusTemporaryRedirect)
}

func recentPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "recent", nil)
}

func tracksPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "tracks", nil)
}

func artistsPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "artists", nil)
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

//
// json data handlers
//
func (app *Application) topArtistsData(w http.ResponseWriter, r *http.Request) {

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

	artists, err := query.TopArtists(app.db.SQL, params)
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

func (app *Application) topNewArtistsData(w http.ResponseWriter, r *http.Request) {

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

	artists, err := query.TopNewArtists(app.db.SQL, params)
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

func (app *Application) topTracksData(w http.ResponseWriter, r *http.Request) {

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

	topTracks, err := query.TopTracks(app.db.SQL, params)
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

func (app *Application) recentTracksData(w http.ResponseWriter, r *http.Request) {

	// don't think i need anything as complicated as the full dateRangeParams here
	// so just use a simple offset/count scheme
	var err error

	type recentTracksResponse struct {
		Offset int                    `json:"offset"`
		Count  int                    `json:"count"`
		Tracks []query.ActivityResult `json:"activity"`
	}

	// get offset & count params, both optional
	offset := 0 // default
	offStr := r.URL.Query().Get("offset")
	if offStr != "" {
		offset, err = strconv.Atoi(offStr)
		if err != nil {
			app.serverError(w, errors.New("error parsing parameter: offset"))
			return
		}
	}
	count := 20 // default
	countStr := r.URL.Query().Get("count")
	if countStr != "" {
		count, err = strconv.Atoi(countStr)
		if err != nil {
			app.serverError(w, errors.New("error parsing parameter: count"))
			return
		}
	}

	recentTracks, err := query.RecentTracks(app.db.SQL, offset, count)
	if err != nil {
		app.serverError(w, err)
		return
	}

	renderJSON(w, http.StatusOK, recentTracksResponse{
		Offset: offset,
		Count:  count,
		Tracks: recentTracks,
	})
}

func (app *Application) listeningClockData(w http.ResponseWriter, r *http.Request) {

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

	clock, err := query.ListeningClock(app.db.SQL, params)
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
