package web

import (
	"log"
	"net/http"
	"path"
	"strings"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/util"

	"github.com/gorilla/sessions"
	"github.com/justinas/alice"
)

type Application struct {
	db           *m.Database
	err          *log.Logger
	info         *log.Logger
	sessionStore *sessions.CookieStore
	Mux          http.Handler

	// updateChan regulates background updates. empty strings
	// written to it indicate timed checks, strings containing
	// usernames are update requests from connected clients
	updateChan chan string

	// synchronized access to map of websocket clients
	// and their update channels
	websocketClients *WebsocketRegistry
}

func CreateApp(db *m.Database, sessionSecret string, info, err *log.Logger) (*Application, error) {

	session := sessions.NewCookieStore([]byte(sessionSecret))
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // XXX config var
		HttpOnly: true,
	}

	app := &Application{
		db:               db,
		info:             info,
		err:              err,
		sessionStore:     session,
		websocketClients: MakeWebsocketRegistry(),
	}

	//
	// create middleware chains to wrap handlers
	//
	standardMiddleware := alice.New(app.logRequest, secureHeaders)
	protectedMiddleware := standardMiddleware.Append(app.requireAuthentication)
	dataMiddleware := standardMiddleware.Append(app.requireAPIAuth)

	//
	// create a new ServeMux and register handlers
	//
	mux := http.NewServeMux()

	// login/logout
	mux.Handle("/login", standardMiddleware.ThenFunc(app.loginUser))
	mux.Handle("/logout", standardMiddleware.ThenFunc(app.logoutUser))

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

	// websocket
	mux.HandleFunc("/ws", app.websocketConnection)

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
