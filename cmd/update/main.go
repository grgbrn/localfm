package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/update"
)

func main() {

	delayPtr := flag.Int("delay", 5, "Delay in seconds between API calls")
	limitPtr := flag.Int("limit", 0, "Limit number of API calls")

	logfilePtr := flag.String("logfile", "", "Log to a file")
	quietPtr := flag.Bool("quiet", false, "Don't print to stdout")

	//dupePtr := flag.Bool("duplicates", false, "Check entire database for duplicates")
	flag.Parse()

	//
	// database init
	//
	DSN := os.Getenv("DSN")
	if DSN == "" {
		panic("Must set DSN environment var")
	}
	db, err := m.Open(DSN)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Connecting to database: %s\n", db.Path)

	//
	// load lastfm credentials
	//
	APIKey := os.Getenv("LASTFM_API_KEY")
	APISecret := os.Getenv("LASTFM_API_SECRET")
	Username := os.Getenv("LASTFM_USERNAME")

	if APIKey == "" || APISecret == "" || Username == "" {
		panic("Must set LASTFM_USERNAME, LASTFM_API_KEY and LASTFM_API_SECRET environment vars")
	}

	//
	// create logger
	//
	var outputs = make([]io.Writer, 0)
	if !*quietPtr {
		outputs = append(outputs, os.Stdout)
	}
	if *logfilePtr != "" {
		f, err := os.Create(*logfilePtr)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		outputs = append(outputs, f)
	}
	tee := io.MultiWriter(outputs...)
	teeLog := log.New(tee, "", log.Ldate|log.Ltime)

	fetcher := update.CreateFetcher(db,
		teeLog,
		update.LastFMCredentials{
			APIKey:    APIKey,
			APISecret: APISecret,
			Username:  Username,
		},
	)

	res, err := fetcher.FetchLatestScrobbles(
		update.FetchOptions{
			APIThrottleDelay: *delayPtr,
			RequestLimit:     *limitPtr,
			CheckDuplicates:  false,
		},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", res)
}
