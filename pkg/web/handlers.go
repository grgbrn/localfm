package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"bitbucket.org/grgbrn/localfm/pkg/query"
)

// login pages
func (app *Application) loginUser(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		app.renderTemplate(w, "login.tmpl", &errorTemplateData{})
	} else if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			app.err.Printf("Login error:%v", err)
			// XXX should be client error
			http.Error(w, "Internal Server Error", http.StatusBadRequest)
			return
		}

		// retrieve field values
		email := r.PostForm.Get("email")
		passwd := r.PostForm.Get("password")
		tz := r.PostForm.Get("tz")

		userID, err := authenticateUser(email, passwd)
		if err != nil {
			app.err.Printf("Authentication error:%v", err)
			http.Error(w, "Internal Server Error", http.StatusBadRequest)
			return
		}
		if userID == -1 {
			app.renderTemplate(w, "login.tmpl", &errorTemplateData{
				Error: "Email or Password is incorrect",
			})
			return
		}
		app.session.Put(r, "authenticatedUserID", userID)
		app.session.Put(r, "timezone", tz)

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
	// XXX shouldn't allow GET but there's no account mgt page yet
	if r.Method != "POST" && r.Method != "GET" {
		http.Error(w, "Internal Server Error", http.StatusBadRequest)
		return
	}
	app.session.Remove(r, "authenticatedUserID")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// authenticated app pages
func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, "./recent", http.StatusTemporaryRedirect)
}

func (app *Application) recentPage(w http.ResponseWriter, r *http.Request, templateName string) {

	offsetParams, err := extractOffsetParams(r)
	if err != nil {
		app.serverError(w, err)
		return
	}

	recentTracks, err := query.RecentTracks(app.db.SQL, offsetParams.Offset, offsetParams.Count)
	if err != nil {
		app.serverError(w, err)
		return
	}

	// not sure this is the best way to handle this, but convert timezone to client localtime
	tz := app.session.GetString(r, "timezone")
	if tz != "" {
		localTZ, err := time.LoadLocation(tz)
		if err != nil {
			app.serverError(w, err)
			return
		}
		for ix, activityResult := range recentTracks {
			recentTracks[ix].Time = activityResult.Time.In(localTZ)
		}
	}

	// figure out previous/next links
	var nextLink, prevLink string
	prevLink = fmt.Sprintf("/htmx/recentTracks?offset=%d&count=%d", offsetParams.Offset+1, offsetParams.Count)
	if offsetParams.Offset > 0 {
		nextLink = fmt.Sprintf("/htmx/recentTracks?offset=%d&count=%d", offsetParams.Offset-1, offsetParams.Count)
	}

	tmp := recentTemplateData{
		PagingData: datebarTemplateData{
			Title:     "Recently Played Tracks",
			Previous:  prevLink,
			Next:      nextLink,
			DOMTarget: "#monthly-pagegrid",
		},

		Tracks: recentTracks,
	}

	app.renderTemplate(w, templateName, tmp)
}

func (app *Application) tracksPage(w http.ResponseWriter, r *http.Request, templateName string) {

	params, err := app.extractDateRangeParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// XXX i have 3 queries to perform here, do them in parallel?

	topTracks, err := query.TopTracks(app.db.SQL, params)
	if err != nil {
		app.serverError(w, err)
		return
	}

	topArtists, err := query.TopNewArtists(app.db.SQL, params)
	if err != nil {
		app.serverError(w, err)
		return
	}

	clock, err := query.ListeningClock(app.db.SQL, params)
	if err != nil {
		app.serverError(w, err)
		return
	}

	// generate next/previous links
	var nextLink, prevLink string
	prevLink = fmt.Sprintf("/htmx/popularTracks?offset=%d&mode=%s", params.Offset+1, params.Mode)
	if params.Offset > 0 {
		nextLink = fmt.Sprintf("/htmx/popularTracks?offset=%d&mode=%s", params.Offset-1, params.Mode)
	}

	// generate title
	var pagingTitle string
	switch params.Mode {
	case "week":
		// mimic javascript toDateString()
		// "Thu Jan 12 2023"
		const dateStringFormat = "Mon Jan 2 2006"
		start := params.Start.Format(dateStringFormat)
		end := params.End.Format(dateStringFormat)
		pagingTitle = start + " to " + end
	case "month":
		pagingTitle = params.Start.Format("Jan 2006")
	case "year":
		pagingTitle = params.Start.Format("2006")
	}

	// listening clock current/avg values
	currentClockValues := make([]int, 24)
	avgClockValues := make([]int, 24)
	for ix, val := range clock {
		currentClockValues[ix] = val.PlayCount
		avgClockValues[ix] = val.AvgCount
	}

	unitTitle := titleCase(params.Mode)

	tmp := trackTemplateData{
		TopTracks:  topTracks,
		TopArtists: topArtists,
		ClockData: clockTemplateData{
			GraphTitle:    unitTitle + "ly Listening Times",
			AvgLabel:      "6 " + unitTitle + " avg",
			CurrentValues: currentClockValues,
			AverageValues: avgClockValues,
		},
		PagingData: datebarTemplateData{
			Title:        "Popular Tracks: " + pagingTitle,
			UnitLabel:    unitTitle,
			DOMTarget:    "#monthly-pagegrid",
			Previous:     prevLink,
			Next:         nextLink,
			DateRangeURL: "/htmx/popularTracks",
		},
	}

	app.renderTemplate(w, templateName, tmp)
}

func (app *Application) artistsPage(w http.ResponseWriter, r *http.Request, templateName string) {

	params, err := app.extractDateRangeParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	artists, err := query.TopArtists(app.db.SQL, params)
	if err != nil {
		app.serverError(w, err)
		return
	}

	// generate next/previous links
	var nextLink, prevLink string
	prevLink = fmt.Sprintf("/htmx/artists?offset=%d&mode=%s", params.Offset+1, params.Mode)
	if params.Offset > 0 {
		nextLink = fmt.Sprintf("/htmx/artists?offset=%d&mode=%s", params.Offset-1, params.Mode)
	}

	// XXX factor this out, used in multiple handlers
	// generate title
	var pagingTitle string
	switch params.Mode {
	case "week":
		// mimic javascript toDateString()
		// "Thu Jan 12 2023"
		const dateStringFormat = "Mon Jan 2 2006"
		start := params.Start.Format(dateStringFormat)
		end := params.End.Format(dateStringFormat)
		pagingTitle = start + " to " + end
	case "month":
		pagingTitle = params.Start.Format("Jan 2006")
	case "year":
		pagingTitle = params.Start.Format("2006")
	}

	unitTitle := titleCase(params.Mode)

	type artistTemplateData struct {
		Artists    []query.ArtistResult
		PagingData datebarTemplateData
	}

	dat := artistTemplateData{
		Artists: artists,
		PagingData: datebarTemplateData{
			Title:        "Recent Artists: " + pagingTitle,
			UnitLabel:    unitTitle,
			DOMTarget:    "#artist-pagegrid",
			Previous:     prevLink,
			Next:         nextLink,
			DateRangeURL: "/htmx/artists",
		},
	}

	app.renderTemplate(w, templateName, dat)
}

func extractOffsetParams(r *http.Request) (query.OffsetParams, error) {
	var err error

	// defaults
	result := query.OffsetParams{
		Offset: 0,
		Count:  20,
	}

	// get offset & count params, both optional
	offStr := r.URL.Query().Get("offset")
	if offStr != "" {
		result.Offset, err = strconv.Atoi(offStr)
		if err != nil {
			return result, errors.New("error parsing parameter: offset")
		}
	}
	countStr := r.URL.Query().Get("count")
	if countStr != "" {
		result.Count, err = strconv.Atoi(countStr)
		if err != nil {
			return result, errors.New("error parsing parameter: count")
		}
	}

	return result, nil
}

// extractDateRangeParams translates mode=X&offset=Y parameters
// from the URL query into start/end/lim parameters expected by
// the query package
func (app *Application) extractDateRangeParams(r *http.Request) (query.DateRangeParams, error) {

	var params query.DateRangeParams

	// required param: mode
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "month"
	}
	params.Mode = mode

	// required param: offset
	offStr := r.URL.Query().Get("offset")
	if offStr == "" {
		offStr = "0"
	}
	offset, err := strconv.Atoi(offStr)
	if err != nil {
		return params, errors.New("invalid format for parameter: offset")
	}
	params.Offset = offset

	// optional param: tz
	// if unset, try the value in the session
	// otherwise default to UTC
	tzStr := r.URL.Query().Get("tz")
	if tzStr == "" {
		tzStr = app.session.GetString(r, "timezone")
	}
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
func (app *Application) topArtistsData(w http.ResponseWriter, r *http.Request) {

	type topArtistsResponse struct {
		Mode      string               `json:"mode"`
		StartDate time.Time            `json:"startDate"`
		EndDate   time.Time            `json:"endDate"`
		Artists   []query.ArtistResult `json:"artists"`
	}

	params, err := app.extractDateRangeParams(r)
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

	params, err := app.extractDateRangeParams(r)
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

	params, err := app.extractDateRangeParams(r)
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

	offsetParams, err := extractOffsetParams(r)
	if err != nil {
		app.serverError(w, err)
		return
	}

	recentTracks, err := query.RecentTracks(app.db.SQL, offsetParams.Offset, offsetParams.Count)
	if err != nil {
		app.serverError(w, err)
		return
	}

	renderJSON(w, http.StatusOK, recentTracksResponse{
		Offset: offsetParams.Offset,
		Count:  offsetParams.Count,
		Tracks: recentTracks,
	})
}

func (app *Application) listeningClockData(w http.ResponseWriter, r *http.Request) {

	type listeningClockResponse struct {
		Mode      string              `json:"mode"`
		StartDate time.Time           `json:"startDate"`
		EndDate   time.Time           `json:"endDate"`
		Clock     []query.ClockResult `json:"clock"`
	}

	params, err := app.extractDateRangeParams(r)
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

func titleCase(s string) string {
	c := cases.Title(language.English)
	return c.String(s)
}

// template structs (move elsewhere?)

// recent.*.tmpl
type recentTemplateData struct {
	PagingData datebarTemplateData

	Tracks []query.ActivityResult
}

// data for datebar.partial.tmpl and nextbar.partial.tmpl
type datebarTemplateData struct {
	Title        string
	UnitLabel    string // XXX document this
	DOMTarget    string
	Previous     string
	Next         string
	DateRangeURL string
}

// listening clock component on tracks.page.tmpl
type clockTemplateData struct {
	GraphTitle string `json:"title"`
	AvgLabel   string `json:"label"`

	CurrentValues []int `json:"currentValues"`
	AverageValues []int `json:"avgValues"`
}

// tracks.page.tmpl
type trackTemplateData struct {
	TopTracks  []query.TrackResult
	TopArtists []query.ArtistResult
	ClockData  clockTemplateData
	PagingData datebarTemplateData
}
