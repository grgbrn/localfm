package web

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

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
			app.err.Printf("Login error:%v", err)
			// XXX should be client error
			http.Error(w, "Internal Server Error", http.StatusBadRequest)
			return
		}

		// retrieve field values
		email := r.PostForm.Get("email")
		passwd := r.PostForm.Get("password")

		userID, err := authenticateUser(email, passwd)
		if err != nil {
			app.err.Printf("Authentication error:%v", err)
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
	// XXX shouldn't allow GET but there's no account mgt page yet
	if r.Method != "POST" && r.Method != "GET" {
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

//
// websocket handler
//
func (app *Application) websocketConnection(w http.ResponseWriter, r *http.Request) {

	// upgrade the GET to a websocket
	upgrader := websocket.Upgrader{}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		app.serverError(w, fmt.Errorf("Error upgrading websocket:%v\n", err))
		return
	}
	defer ws.Close()

	// register the new client
	wc := WebsocketClient{
		RemoteAddress: ws.RemoteAddr(),
		Username:      "grgbrn", // XXX
		UserAgent:     r.UserAgent(),
		ConnectedAt:   time.Now(),
		LastActivity:  time.Now(),
	}
	app.info.Printf("new client:%+v\n", wc)

	// XXX wrap this up
	app.websocketClients.Lock()
	app.websocketClients.m[ws] = wc
	app.websocketClients.Unlock()
	app.info.Printf("total clients:%d\n", len(app.websocketClients.m))

	// this goroutine is responsible for registering/unregistering
	// it's interest in updates for a specific user
	// it's this call that increases the freqency of polling for that user (TODO)
	updateChan, err := app.registerForUpdates(wc)
	if err != nil {
		panic("can't register client, not sure how to handle this")
	}
	defer app.deregisterForUpdates(wc)

	// when it gets updates for that user, it needs to send them to the browser
	// it can also get requests from the browser to do an immediate update
	// only way to do this in go is select on channels, so the blocking reads
	// of the websocket API need to turn into channel operations
	clientMsgChan, clientErrChan := readFromWebSocket(ws)

	clientActive := true
	for clientActive {
		var updateNotification string
		var clientMessage Message
		var clientError error

		select {
		case updateNotification = <-updateChan:
			fmt.Println("== got an update notification")
			fmt.Println(updateNotification)
			err := ws.WriteJSON(Message{
				Username: "Server",
				Message:  updateNotification,
			})
			if err != nil {
				// XXX should probably close and cleanup?
				fmt.Println("!!! error writing message to client")
			}

		case clientMessage = <-clientMsgChan:
			fmt.Println("=== got a client message")
			fmt.Println(clientMessage)
			if clientMessage.Message == "refresh" {
				// post something to the update channel and see
				// if it passes the minimum filter
				// XXX would be nice to return an error to the client
				// XXX if it doesn't work
				app.updateChan <- true
			} else {
				fmt.Printf("ignoring unknown client message:%s\n", clientMessage.Message)
			}
		case clientError = <-clientErrChan:
			app.info.Printf("removing client %s err=%v\n", wc, clientError)

			// XXX wrap this up
			app.websocketClients.Lock()
			delete(app.websocketClients.m, ws)
			app.websocketClients.Unlock()

			clientActive = false
		}
		// XXX not sure if this is safe or not
		wc.LastActivity = time.Now() // XXX race condition?
	}
	// XXX what kind of cleanup needs to happen here?
}

// read messages from the websocket and convert them to a channel
// (so they can be used with select)
// creates a new goroutine in the background which exits when the
// websocket connection is closed
func readFromWebSocket(ws *websocket.Conn) (chan Message, chan error) {
	msgChan := make(chan Message)
	errChan := make(chan error)

	go func() {
		done := false
		for !done {
			var msg Message
			// read in a new message as json and map it to a Message
			err := ws.ReadJSON(&msg)
			if err != nil {
				errChan <- err
				done = true
			} else {
				msgChan <- msg
			}
		}
		fmt.Println("readFromWebsocket goroutine exiting")
		close(msgChan)
		close(errChan)
	}()

	return msgChan, errChan
}

// XXX sample message
// XXX add timestamp? localtime? timezone?
type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type WebsocketClient struct {
	RemoteAddress net.Addr
	UserAgent     string

	Username     string
	ConnectedAt  time.Time
	LastActivity time.Time
}

func (c WebsocketClient) String() string {
	return fmt.Sprintf("%s@%s", c.Username, c.RemoteAddress)
}
