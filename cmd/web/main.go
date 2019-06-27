package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/util"
)

type application struct {
	db   *sql.DB
	err  *log.Logger
	info *log.Logger
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

	//
	// Initialize a new instance of application containing the dependencies.
	//
	app := &application{
		db:   db,
		info: infoLog,
		err:  errorLog,
	}

	//
	// create a new ServeMux and register handlers
	//
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)

	mux.HandleFunc("/recent", recentPage)
	mux.HandleFunc("/monthly", monthlyPage)
	mux.HandleFunc("/artists", artistsPage)

	mux.HandleFunc("/data/artists", app.topArtistsData)
	mux.HandleFunc("/data/monthlyArtists", app.monthlyArtistData)
	mux.HandleFunc("/data/monthlyTracks", app.monthlyTrackData)
	mux.HandleFunc("/data/listeningClock", app.listeningClockData)

	// set up static file server to ignore /ui/static/ prefix
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	addr := ":4000" // XXX
	srv := &http.Server{
		Addr:     addr,
		ErrorLog: app.err,
		Handler:  app.logRequest(secureHeaders(mux)),
	}

	app.info.Printf("Starting server on %s\n", addr)
	err = srv.ListenAndServe()
	app.err.Fatal(err)
}
