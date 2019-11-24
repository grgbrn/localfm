package web

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"bitbucket.org/grgbrn/localfm/pkg/update"
	"github.com/gorilla/websocket"
)

const maxUpdateFrequencyMinutes float64 = 5

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

// PeriodicUpdate is intended to be called in a long-running goroutine that
// will occasionally call update to fetch new data from lastfm
func (app *Application) PeriodicUpdate(updateFreq int, baseLogDir string, credentials update.LastFMCredentials) error {
	if app.updateChan != nil {
		return errors.New("PeriodicUpdate can only be started once")
	}
	app.updateChan = make(chan bool)

	// goroutine that ticks every N minutes (replaces cron)
	go func() {
		ticker := time.NewTicker(time.Duration(updateFreq) * time.Minute)
		for {
			<-ticker.C
			app.info.Println("Starting periodic update")
			app.updateChan <- true
		}
	}()
	app.info.Printf("Starting periodic updates every %d min\n", updateFreq)

	var lastRun time.Time

	for {
		// wait for someone to post to the update channel
		<-app.updateChan

		// simple throttle
		if !lastRun.IsZero() && time.Since(lastRun).Minutes() < maxUpdateFrequencyMinutes {
			app.info.Printf("Ignoring update request, most recent was only %f minutes ago (%f minimum)\n",
				time.Since(lastRun).Minutes(), maxUpdateFrequencyMinutes)
			continue
		}

		// XXX for now don't actually run updates
		// app.doUpdate(baseLogDir, credentials) // don't do anyting special on error

		fmt.Println("XXX simulating update for grgbrn")
		updateResult := fmt.Sprintf("[ update at %s ]", time.Now().String())

		// write the update to any registered channels
		for client, updateChan := range app.registeredClients {
			fmt.Printf("sending update to registered client:%s\n", client)
			updateChan <- updateResult
		}

		lastRun = time.Now()
	}
}

func (app *Application) doUpdate(baseLogDir string, credentials update.LastFMCredentials) error {

	// create a datestamped logfile in our logdir for this update
	// 2019/11/03 16:05:44  ->  20191103_160544
	dateSegment := time.Now().Format("20060102_150405")
	logPath := path.Join(baseLogDir, fmt.Sprintf("%s.log", dateSegment))

	app.info.Printf("Logging to %s\n", logPath)

	f, err := os.Create(logPath)
	if err != nil {
		return err
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
		return err
	}
	app.info.Println("Update succeeded")
	app.info.Printf("%+v\n", res)

	// this will need to be able to return some kind of meaningful client info!
	return nil
}

func (app *Application) registerForUpdates(client WebsocketClient) (chan string, error) {
	// this fn needs to increase the fetch frequency for the client
	// and create a new channel that will get a message when there
	// are updates for that user
	fmt.Printf(">> registering client:%s\n", client)
	ch := make(chan string)
	app.registeredClients[client] = ch
	fmt.Printf(">> %d active clients\n", len(app.registeredClients))
	return ch, nil
}

func (app *Application) deregisterForUpdates(client WebsocketClient) error {
	// this fn needs to reduce the fetch frequency for the client
	// and clean up / deregister the channel
	fmt.Printf(">> deregistering client:%s\n", client)
	ch := app.registeredClients[client]
	delete(app.registeredClients, client)
	close(ch)
	fmt.Printf(">> %d clients remaining\n", len(app.registeredClients))
	return nil
}
