package web

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/update"
	"bitbucket.org/grgbrn/localfm/pkg/util"

	"github.com/golangcollege/sessions"
	"github.com/justinas/alice"
)

type Application struct {
	db      *m.Database
	err     *log.Logger
	info    *log.Logger
	session *sessions.Session
	Mux     http.Handler

	updateChan chan bool
}

func CreateApp(db *m.Database, sessionSecret string, info, err *log.Logger) (*Application, error) {

	session := sessions.New([]byte(sessionSecret))
	session.Lifetime = 24 * 7 * time.Hour

	//
	// Initialize a new instance of application containing the dependencies.
	//
	app := &Application{
		db:      db,
		info:    info,
		err:     err,
		session: session,
	}

	//
	// create middleware chains
	//
	standardMiddleware := alice.New(app.logRequest, secureHeaders)
	dynamicMiddleware := standardMiddleware.Append(app.session.Enable)
	protectedMiddleware := dynamicMiddleware.Append(app.requireAuthentication)
	dataMiddleware := dynamicMiddleware.Append(app.requireAPIAuth)

	//
	// create a new ServeMux and register handlers
	//
	mux := http.NewServeMux()

	// login/logout
	mux.Handle("/login", dynamicMiddleware.ThenFunc(app.loginUser))
	mux.Handle("/logout", dynamicMiddleware.ThenFunc(app.logoutUser))

	// app pages
	mux.Handle("/", protectedMiddleware.ThenFunc(index))
	mux.Handle("/recent", protectedMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.recentPage(w, r, "recent.page.tmpl")
	}))
	mux.Handle("/tracks", protectedMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.tracksPage(w, r, "tracks.page.tmpl")
	}))
	mux.Handle("/artists", protectedMiddleware.ThenFunc(app.artistsPage))

	// htmx calls
	mux.Handle("/htmx/recentTracks", dataMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.recentPage(w, r, "recent.tile.tmpl")
	}))
	mux.Handle("/htmx/popularTracks", dataMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.tracksPage(w, r, "tracks.tile.tmpl")
	}))
	mux.Handle("/htmx/artists", dataMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "XXX not yet implemented", http.StatusBadRequest)
	}))

	// data calls
	mux.Handle("/data/topArtists", dataMiddleware.ThenFunc(app.topArtistsData))
	mux.Handle("/data/topNewArtists", dataMiddleware.ThenFunc(app.topNewArtistsData))
	mux.Handle("/data/topTracks", dataMiddleware.ThenFunc(app.topTracksData))
	mux.Handle("/data/listeningClock", dataMiddleware.ThenFunc(app.listeningClockData))
	mux.Handle("/data/recentTracks", dataMiddleware.ThenFunc(app.recentTracksData))

	// set up static file server to ignore /ui/static/ prefix
	fileRoot := util.GetEnvStr("STATIC_FILE_ROOT", ".")
	prefix := path.Join(fileRoot, "ui/static/")
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	fileServer := http.FileServer(http.Dir(prefix))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	app.Mux = standardMiddleware.Then(mux)

	return app, nil
}

const maxUpdateFrequencyMinutes float64 = 5

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

		app.doUpdate(baseLogDir, credentials) // don't do anyting special on error
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
