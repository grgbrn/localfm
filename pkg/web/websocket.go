package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"bitbucket.org/grgbrn/localfm/pkg/update"
	"github.com/gorilla/websocket"
)

// WebsocketClient represents a connected browser listening
// for updates
type WebsocketClient struct {
	RemoteAddress net.Addr
	UserAgent     string

	Username     string
	ConnectedAt  time.Time
	LastActivity time.Time

	conn *websocket.Conn // not sure this is needed
}

func (c WebsocketClient) String() string {
	return fmt.Sprintf("%s@%s", c.Username, c.RemoteAddress)
}

type WebsocketMessage struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

// WebsocketRegistry provides synchronized access to all
// connected clients and tracks their update channels
type WebsocketRegistry struct {
	sync.RWMutex
	m map[WebsocketClient]chan string
}

// Register adds a new client and it's update channel to the registry
func (reg *WebsocketRegistry) Register(client WebsocketClient, updateChan chan string) int {
	reg.Lock()
	reg.m[client] = updateChan
	count := len(reg.m)
	reg.Unlock()
	return count
}

// Deregister removes a client and closes it's update channel
func (reg *WebsocketRegistry) Deregister(client WebsocketClient) int {
	reg.Lock()
	ch, exists := reg.m[client]
	if exists {
		close(ch)
		delete(reg.m, client)
	}
	count := len(reg.m)
	reg.Unlock()
	return count
}

// ChannelsForUsername returns all update channels that are listening
// for updates for a given username
func (reg *WebsocketRegistry) ChannelsForUsername(username string) []chan string {
	channels := make([]chan string, 0)
	reg.RLock()
	for client, updateChan := range reg.m {
		if client.Username == username {
			channels = append(channels, updateChan)
		}
	}
	reg.RUnlock()
	return channels
}

// Count gives the number of currently connected clients
func (reg *WebsocketRegistry) Count() int {
	reg.RLock()
	count := len(reg.m)
	reg.RUnlock()
	return count
}

// MakeWebsocketRegistry creates a new WebsocketRegistry
func MakeWebsocketRegistry() *WebsocketRegistry {
	return &WebsocketRegistry{
		m: make(map[WebsocketClient]chan string),
	}
}

// websocketConnection is the http.Handler that is run for long-running
// websocket connections. It registers the client and adds it's update channel
// to the list of active clients, in order for it to receive notification of
// new items. This handler and it's goroutine run for as long as the
// websocket remains open
func (app *Application) websocketConnection(w http.ResponseWriter, r *http.Request) {

	// regular session still exists before upgrading the websocket
	var currentUsername string
	if app.isAuthenticated(r) {
		currentUsername = "grgbrn"
	} else {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

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
		Username:      currentUsername,
		UserAgent:     r.UserAgent(),
		ConnectedAt:   time.Now(),
		LastActivity:  time.Now(),
		conn:          ws,
	}
	app.info.Printf("new client:%+v\n", wc)

	// make a channel the client can use to listen
	// for updates
	updateChan := make(chan string)

	c := app.websocketClients.Register(wc, updateChan)
	app.info.Printf("total clients:%d\n", c)

	// Listen for both messages from the client and events on
	// the update channel. This requires doing a select on
	// multiple channels, so the websocket must be wrapped
	// in a channel
	clientMsgChan, clientErrChan := readFromWebSocket(ws)

	clientActive := true
	for clientActive {
		var updateNotification string
		var clientMessage WebsocketMessage
		var clientError error

		select {
		case updateNotification = <-updateChan:
			app.info.Printf("Got update notification:%s\n", updateNotification)
			err := ws.WriteJSON(WebsocketMessage{
				Username: "Server",
				Message:  updateNotification,
			})
			if err != nil {
				app.info.Printf("removing client %s err=%v\n", wc, err)
				// cleanup and remove client
				app.websocketClients.Deregister(wc)
				clientActive = false
			}

		case clientMessage = <-clientMsgChan:
			app.info.Printf("Got client message: %s\n", clientMessage)
			// only one valid client message at the moment
			if clientMessage.Message == "refresh" {
				// this update request may not be triggered if
				// it's below the minimum update threshold
				// would be nice to return a notice to the client
				// if the update is rejected
				app.updateChan <- currentUsername
			} else {
				app.info.Printf("ignoring unknown client message:%s\n", clientMessage.Message)
			}
		case clientError = <-clientErrChan:
			app.info.Printf("removing client %s err=%v\n", wc, clientError)
			app.websocketClients.Deregister(wc)
			clientActive = false
		}
		wc.LastActivity = time.Now()
	}
}

// readFromWebSocket reads messages from the websocket connection
// and converts them to messages on a channel. Any messages received
// over the websocket connection are parsed and sent over the
// WebsocketMessage channel, the error channel is used to signal that
// the websocket connection is closing down.
// This function creates a goroutine which exits when the websocket
// connection is closed
func readFromWebSocket(ws *websocket.Conn) (chan WebsocketMessage, chan error) {
	msgChan := make(chan WebsocketMessage)
	errChan := make(chan error)

	go func() {
		done := false
		for !done {
			var msg WebsocketMessage
			// read in a new message as json and map it to a Message
			err := ws.ReadJSON(&msg)
			// XXX does this mean a parse error can close the connection?
			// XXX how to distinguish the two?
			if err != nil {
				errChan <- err
				done = true
			} else {
				msgChan <- msg
			}
		}
		close(msgChan)
		close(errChan)
	}()

	return msgChan, errChan
}

// hard throttle limit on how often we'll trigger an update
// (note that this is unrelated to api call throttle!)
const maxUpdateFrequencyMinutes float64 = 2

type PrintFunc func(string, ...interface{})

func PrintNoOp(fmt string, v ...interface{}) {}

const verboseDebugging = false // XXX config

// PeriodicUpdate runs in a long-running goroutine and triggers calls
// against the lastfm api to find updated tracks. The frequency of thse
// updates depends on whether a user is connected and actively receiving
// notifications of new items.
//
// app.updateChan regulates these updates. It is triggered at a fixed
// time interval to check if the user is due for an update.
// A connected client may also post to updateChan to request an immediate
// update, which will be rejected only if the most recent update
// was less than maxUpdateFrequencyMinutes ago.
func (app *Application) PeriodicUpdate(updateFreq int, baseLogDir string, credentials update.LastFMCredentials) error {
	if app.updateChan != nil {
		return errors.New("PeriodicUpdate can only be started once")
	}
	app.updateChan = make(chan string)

	inactiveDuration := time.Duration(updateFreq) * time.Minute
	activeDuration := time.Duration(10) * time.Minute // XXX this should be configurable too

	var debug PrintFunc = PrintNoOp
	if verboseDebugging {
		debug = app.info.Printf
	}

	// tick every minute. do this instead of time.sleep() so a connected
	// user can trigger a wakeup
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for {
			<-ticker.C
			debug("tick")
			app.updateChan <- ""
		}
	}()

	app.info.Printf("Starting updates, active=%s, inactive=%s\n", activeDuration, inactiveDuration)

	lastRunTimes, err := loadLastRunTimes()
	if err != nil {
		app.info.Printf("Couldn't load last run times:%v", err)
		lastRunTimes = make(map[string]time.Time)
	}
	app.info.Printf("Loaded %d last run times", len(lastRunTimes))

	for {
		// block on the update channel
		updateRequest := <-app.updateChan

		// if it's a user-generated request, update that user
		// otherwise find the user who hasn't been updated in the
		// longest time and see if they're due
		// (active users should get priority though)
		// XXX this doesn't matter until there are actually multiple users
		userRequestedUpdate := len(updateRequest) > 0
		userToUpdate := "grgbrn" // MULTIUSER

		lastRun := lastRunTimes[userToUpdate]
		timeSinceLastRun := time.Since(lastRun)

		debug("user=%s  lastRun=%v  ago=%v", userToUpdate, lastRun, timeSinceLastRun)

		// simple throttle for maxUpdateFrequencyMinutes
		if timeSinceLastRun.Minutes() < maxUpdateFrequencyMinutes {
			// only log a message if it's a user-generated request
			if userRequestedUpdate {
				app.info.Printf("Ignoring update request, most recent was only %f minutes ago (%f minimum)\n",
					timeSinceLastRun.Minutes(), maxUpdateFrequencyMinutes)
			}
			continue // next loop iteration
		}

		// see if it's time to run an update again, based on whether
		// the user is active (only one user for now, so simple)
		currentUpdateDuration := inactiveDuration
		if app.websocketClients.Count() > 0 {
			currentUpdateDuration = activeDuration
		}
		debug("current update frequency:%v\n", currentUpdateDuration)

		if timeSinceLastRun <= currentUpdateDuration {
			debug("no user due for update")
			continue // next loop iteration
		}

		res, err := app.doUpdate(baseLogDir, credentials)
		if err != nil {
			// Update has failed, so nothing to send.
			// XXX would an error notification to the client be useful?
		}
		if res.NewItems > 0 {
			updateResult := fmt.Sprintf("%d new items available", res.NewItems)

			// write the update to any registered channels
			userChannels := app.websocketClients.ChannelsForUsername(userToUpdate)
			fmt.Printf("sending update to %d registered clients\n", len(userChannels))
			for _, ch := range userChannels {
				ch <- updateResult
			}
		}

		lastRunTimes[userToUpdate] = time.Now()
		saveLastRunTimes(lastRunTimes)
	}
}

// doUpdate triggers an update and returns a FetchResults, which summarizes the
// number of new items found.
func (app *Application) doUpdate(baseLogDir string, credentials update.LastFMCredentials) (*update.FetchResults, error) {

	// create a datestamped logfile in our logdir for this update
	// 2019/11/03 16:05:44  ->  20191103_160544
	dateSegment := time.Now().Format("20060102_150405")
	logPath := path.Join(baseLogDir, fmt.Sprintf("%s.log", dateSegment))

	app.info.Printf("Logging to %s\n", logPath)

	f, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	logger := log.New(f, "", log.Ldate|log.Ltime)

	fetcher := update.CreateFetcher(app.db, logger, credentials)

	res, err := fetcher.FetchLatestScrobbles(
		update.FetchOptions{
			APIThrottleDelay: 5, // XXX
			RequestLimit:     0, // XXX
			CheckDuplicates:  false,
		},
	)
	if err != nil {
		app.info.Println("Update failed")
		app.info.Println(err)
		return nil, err
	}
	app.info.Println("Update succeeded")
	app.info.Printf("%+v\n", res)

	return &res, nil
}

//
// temporary until user db comes along
//

// name of the filename to store last run times
const lastRunFilename = "lastrun.json"

func loadLastRunTimes() (map[string]time.Time, error) {
	runtimes := make(map[string]time.Time)

	dat, err := ioutil.ReadFile(lastRunFilename)
	if err != nil {
		return runtimes, err
	}

	if err := json.Unmarshal(dat, &runtimes); err != nil {
		return runtimes, err
	}
	return runtimes, nil
}

func saveLastRunTimes(data map[string]time.Time) error {
	jout, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(lastRunFilename, jout, 0644)
	if err != nil {
		return err
	}
	return nil
}
