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
	"time"

	"bitbucket.org/grgbrn/localfm/pkg/update"
	"github.com/gorilla/websocket"
)

// hard throttle limit on how often we'll trigger an update
// (note that this is unrelated to api call throttle!)
const maxUpdateFrequencyMinutes float64 = 2

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

// XXX sample message
// XXX add timestamp? localtime? timezone?
type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type PrintFunc func(string, ...interface{})

func PrintNoOp(fmt string, v ...interface{}) {}

const verboseDebugging = false // XXX config

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

	// XXX need to figure this out somehow
	currentUsername := "grgbrn"

	// register the new client
	wc := WebsocketClient{
		RemoteAddress: ws.RemoteAddr(),
		Username:      currentUsername,
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
			app.info.Printf("Got update notification:%s\n", updateNotification)
			err := ws.WriteJSON(Message{
				Username: "Server",
				Message:  updateNotification,
			})
			if err != nil {
				// XXX should probably close and cleanup?
				fmt.Println("!!! error writing message to client")
			}

		case clientMessage = <-clientMsgChan:
			app.info.Printf("Got client message: %s\n", clientMessage)
			if clientMessage.Message == "refresh" {
				// writing a string containing a username to the
				// update channel triggers an update if it's
				// below the minimum update threshold
				// XXX would be nice to return an error to the client
				// XXX if the update isn't going to happen
				app.updateChan <- currentUsername
			} else {
				app.info.Printf("ignoring unknown client message:%s\n", clientMessage.Message)
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
		//fmt.Println("readFromWebsocket goroutine exiting")
		close(msgChan)
		close(errChan)
	}()

	return msgChan, errChan
}

// PeriodicUpdate is intended to be called in a long-running goroutine that
// will occasionally call update to fetch new data from lastfm
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

	// goroutine that ticks every minute. do this instead of time.sleep
	// so that a connected user can trigger a wakeup
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
		// wait for someone to post to the update channel
		updateRequest := <-app.updateChan

		// if it's a user-generated request, update that user
		// otherwise find the user who hasn't been updated in the
		// longest time and see if they're due
		// (active users should get priority though)
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
		if len(app.registeredClients) > 0 {
			currentUpdateDuration = activeDuration
		}
		debug("current update frequency:%v\n", currentUpdateDuration)

		if timeSinceLastRun <= currentUpdateDuration {
			debug("no user due for update")
			continue // next loop iteration
		}

		res, err := app.doUpdate(baseLogDir, credentials)
		if err != nil {
			// if the update fails we have nothing to send
			// not sure if an error notification to the client
			// would be useful here
		}
		if res.NewItems > 0 {
			updateResult := fmt.Sprintf("%d new items available", res.NewItems)

			// write the update to any registered channels
			// MULTIUSER filter this by users
			for client, updateChan := range app.registeredClients {
				fmt.Printf("sending update to registered client:%s\n", client)
				updateChan <- updateResult
			}
		}

		lastRunTimes[userToUpdate] = time.Now()
		saveLastRunTimes(lastRunTimes)
	}
}

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

	// this will need to be able to return some kind of meaningful client info!
	return &res, nil
}

func (app *Application) registerForUpdates(client WebsocketClient) (chan string, error) {
	// this fn needs to increase the fetch frequency for the client
	// and create a new channel that will get a message when there
	// are updates for that user
	// fmt.Printf(">> registering client:%s\n", client)
	ch := make(chan string)
	app.registeredClients[client] = ch
	// fmt.Printf(">> %d active clients\n", len(app.registeredClients))
	return ch, nil
}

func (app *Application) deregisterForUpdates(client WebsocketClient) error {
	// this fn needs to reduce the fetch frequency for the client
	// and clean up / deregister the channel
	// fmt.Printf(">> deregistering client:%s\n", client)
	ch := app.registeredClients[client]
	delete(app.registeredClients, client)
	close(ch)
	// fmt.Printf(">> %d clients remaining\n", len(app.registeredClients))
	return nil
}

//
// temporary until user db comes along
//
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
