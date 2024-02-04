package web

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	m "bitbucket.org/grgbrn/localfm/pkg/model"

	"github.com/golangcollege/sessions"
	"github.com/justinas/alice"
)

type Application struct {
	db            *m.Database
	err           *log.Logger
	info          *log.Logger
	session       *sessions.Session
	Mux           http.Handler
	templateCache map[string]*template.Template
}

func CreateApp(db *m.Database, staticFileRoot string, sessionSecret string, info, errorLog *log.Logger) (*Application, error) {

	session := sessions.New([]byte(sessionSecret))
	session.Lifetime = 24 * 7 * time.Hour

	templateCache, err := newTemplateCache()
	if err != nil {
		return nil, err
	}

	//
	// Initialize a new instance of application containing the dependencies.
	//
	app := &Application{
		db:            db,
		info:          info,
		err:           errorLog,
		session:       session,
		templateCache: templateCache,
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
		app.recentPage(w, r, "recent.tmpl")
	}))
	mux.Handle("/tracks", protectedMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.tracksPage(w, r, "tracks.tmpl")
	}))
	mux.Handle("/artists", protectedMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.artistsPage(w, r, "artists.tmpl")
	}))

	// htmx calls
	mux.Handle("/htmx/recentTracks", dataMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.recentPage(w, r, "recent-fragment.tmpl")
	}))
	mux.Handle("/htmx/popularTracks", dataMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.tracksPage(w, r, "tracks-fragment.tmpl")
	}))
	mux.Handle("/htmx/artists", dataMiddleware.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		app.artistsPage(w, r, "artists-fragment.tmpl")
	}))

	// data calls
	mux.Handle("/data/topArtists", dataMiddleware.ThenFunc(app.topArtistsData))
	mux.Handle("/data/topNewArtists", dataMiddleware.ThenFunc(app.topNewArtistsData))
	mux.Handle("/data/topTracks", dataMiddleware.ThenFunc(app.topTracksData))
	mux.Handle("/data/listeningClock", dataMiddleware.ThenFunc(app.listeningClockData))
	mux.Handle("/data/recentTracks", dataMiddleware.ThenFunc(app.recentTracksData))

	// set up static file server to ignore /ui/static/ prefix
	prefix := path.Join(staticFileRoot, "ui/static/")
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	fileServer := http.FileServer(http.Dir(prefix))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	app.Mux = standardMiddleware.Then(mux)

	return app, nil
}
