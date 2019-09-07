package main

import (
	"crypto/rand"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golangcollege/sessions"
	"github.com/justinas/alice"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/util"
)

type application struct {
	db      *sql.DB
	err     *log.Logger
	info    *log.Logger
	session *sessions.Session
}

func main() {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	//
	// database init (could be factored out!)
	//
	DSN := os.Getenv("DSN")
	if DSN == "" {
		panic("Must set DSN environment var")
	}
	// mimic DSN format from earlier python version of this tool
	// "sqlite:///foo.db"
	if !strings.HasPrefix(DSN, "sqlite://") {
		panic("DSN var must be of the format 'sqlite:///foo.db'")
	}
	dbPath := DSN[9:]

	// sqlite database drivers will automatically create empty databases
	// if the file doesn't exist, so stat the file first and abort
	// if there's no database (must be manually created with schema)
	if !util.FileExists(dbPath) {
		panic("Can't open database [0]")
	}

	// this seemingly never returns an error
	db, err := m.InitDB(dbPath)
	if err != nil {
		panic("Can't open database [1]")
	}

	// init session store
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		infoLog.Println("SESSION_SECRET not set, using temporary value")

		key := [32]byte{}
		_, err := rand.Read(key[:])
		if err != nil {
			panic(err) // XXX
		}
		sessionSecret = string(key[:])
	}
	if len(sessionSecret) != 32 {
		panic("SESSION_SECRET must contain 32 bytes")
	}
	session := sessions.New([]byte(sessionSecret))
	session.Lifetime = 12 * time.Hour

	//
	// Initialize a new instance of application containing the dependencies.
	//
	app := &application{
		db:      db,
		info:    infoLog,
		err:     errorLog,
		session: session,
	}

	//
	// create middleware chains
	//
	standardMiddleware := alice.New(app.logRequest, secureHeaders)
	dynamicMiddleware := alice.New(app.session.Enable)

	//
	// create a new ServeMux and register handlers
	//
	mux := http.NewServeMux()
	mux.Handle("/", dynamicMiddleware.ThenFunc(index))

	mux.Handle("/recent", dynamicMiddleware.ThenFunc(recentPage))
	mux.Handle("/tracks", dynamicMiddleware.ThenFunc(tracksPage))
	mux.Handle("/artists", dynamicMiddleware.ThenFunc(artistsPage))

	mux.Handle("/data/topArtists", dynamicMiddleware.ThenFunc(app.topArtistsData))
	mux.Handle("/data/topNewArtists", dynamicMiddleware.ThenFunc(app.topNewArtistsData))
	mux.Handle("/data/topTracks", dynamicMiddleware.ThenFunc(app.topTracksData))
	mux.Handle("/data/listeningClock", dynamicMiddleware.ThenFunc(app.listeningClockData))
	mux.Handle("/data/recentTracks", dynamicMiddleware.ThenFunc(app.recentTracksData))

	// set up static file server to ignore /ui/static/ prefix
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	addr := ":4000" // XXX
	srv := &http.Server{
		Addr:     addr,
		ErrorLog: app.err,
		Handler:  standardMiddleware.Then(mux),
	}

	app.info.Printf("Starting server on %s\n", addr)
	err = srv.ListenAndServe()
	app.err.Fatal(err)
}
