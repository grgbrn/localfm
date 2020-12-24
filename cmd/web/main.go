package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/update"
	"bitbucket.org/grgbrn/localfm/pkg/util"
	"bitbucket.org/grgbrn/localfm/pkg/web"
)

func main() {
	noupdatePtr := flag.Bool("noupdate", false, "Don't start the database update goroutine")
	flag.Parse()

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	// database init
	db, err := model.Open(util.MustGetEnvStr("DSN"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	infoLog.Printf("Connecting to database: %s\n", db.Path)

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

	// create webapp
	app, err := web.CreateApp(db, sessionSecret, infoLog, errorLog)
	if err != nil {
		panic(err)
	}

	// load lastfm credentials
	lastfmCreds := update.LastFMCredentials{
		APIKey:    util.MustGetEnvStr("LASTFM_API_KEY"),
		APISecret: util.MustGetEnvStr("LASTFM_API_SECRET"),
		Username:  util.MustGetEnvStr("LASTFM_USERNAME"),
	}

	// start goroutine to kick off periodic updates of lastfm data
	if !*noupdatePtr {
		var updateLogDir = util.GetEnvStr("UPDATE_LOGIDR", "/tmp/updatelogs")
		err = os.MkdirAll(updateLogDir, 0755)
		if err != nil {
			panic(err)
		}
		go app.PeriodicUpdate(
			util.GetEnvInt("UPDATE_FREQUENCY_MINUTES", 60),
			updateLogDir,
			lastfmCreds)
	} else {
		infoLog.Println("Not starting periodic update task")
	}

	// create & run the webserver on the main goroutine
	addr := fmt.Sprintf(":%d", util.GetEnvInt("HTTP_PORT", 4000))
	srv := &http.Server{
		Addr:     addr,
		ErrorLog: errorLog,
		Handler:  app.Mux,
	}

	infoLog.Printf("Starting server on %s\n", addr)
	err = srv.ListenAndServe()
	errorLog.Fatal(err)
}
