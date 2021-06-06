package web

import (
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
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
	mux.Handle("/recent", protectedMiddleware.ThenFunc(recentPage))
	mux.Handle("/tracks", protectedMiddleware.ThenFunc(tracksPage))
	mux.Handle("/artists", protectedMiddleware.ThenFunc(artistsPage))

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
