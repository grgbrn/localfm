package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/update"
	"bitbucket.org/grgbrn/localfm/pkg/util"
	"bitbucket.org/grgbrn/localfm/pkg/web"
)

// main entry point for the webapp
// can optionally run the update in-process in a goroutine
func main() {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	// database init
	db, err := model.Open(util.MustGetEnvStr("DSN"))
	if err != nil {
		panic(err)
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

	fileRoot := util.GetEnvStr("STATIC_FILE_ROOT", ".")

	// create webapp
	app, err := web.CreateApp(db, fileRoot, sessionSecret, infoLog, errorLog)
	if err != nil {
		panic(err)
	}

	// load lastfm credentials
	lastfmCreds := update.LastFMCredentials{
		APIKey:    util.MustGetEnvStr("LASTFM_API_KEY"),
		APISecret: util.MustGetEnvStr("LASTFM_API_SECRET"),
		Username:  util.MustGetEnvStr("LASTFM_USERNAME"),
	}

	updateFreq := os.Getenv("UPDATE_FREQUENCY_MINUTES")
	if updateFreq != "" {
		i, err := strconv.Atoi(updateFreq)
		if err != nil {
			panic(fmt.Sprintf("Error parsing UPDATE_FREQUENCY_MINUTES as an int: %s", updateFreq))
		}
		updateFreq := time.Duration(i) * time.Minute
		ticker := time.NewTicker(updateFreq)

		// fetch options can be overriden from environment
		throttleDelay := time.Duration(util.GetEnvInt("API_THROTTLE_DELAY", 5)) * time.Second
		requestLimit := util.GetEnvInt("API_REQUEST_LIMIT", 0)

		// start goroutine to kick off periodic updates of lastfm data
		go func() {
			infoLog.Printf("Starting periodic updates every %v\n", updateFreq)

			for {
				// wait for the next tick to run
				<-ticker.C

				infoLog.Println("Doing periodic update")

				fetcher := update.CreateFetcher(db, infoLog, lastfmCreds)

				res, err := fetcher.FetchLatestScrobbles(
					update.FetchOptions{
						APIThrottleDelay: throttleDelay,
						RequestLimit:     requestLimit,
					},
				)
				if err != nil {
					infoLog.Println("Update failed")
					infoLog.Println(err)
				}
				infoLog.Println("Update succeeded")
				infoLog.Printf("%+v\n", res)
			}

		}()
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
