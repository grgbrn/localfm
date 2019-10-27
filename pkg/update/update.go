package update

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/util"

	"github.com/shkh/lastfm-go/lastfm"
)

// Fetcher contains all of the context necessary to download new scrobbles
type Fetcher struct {
	db       *m.Database
	log      log.Logger
	creds    LastFMCredentials
	lastfm   *lastfm.Api
	throttle <-chan time.Time
}

// CreateFetcher makes a new Fetcher
func CreateFetcher(db *m.Database, creds LastFMCredentials) *Fetcher {
	api := lastfm.New(creds.APIKey, creds.APISecret)

	return &Fetcher{
		db:     db,
		creds:  creds,
		lastfm: api,
	}
}

// LastFMCredentials holds all of a user's API login credentials
type LastFMCredentials struct {
	APIKey    string
	APISecret string
	Username  string
}

// FetchOptions is all possible flags to pass to FetchLatestScrobbles
type FetchOptions struct {
	APIThrottleDelay int
	RequestLimit     int
	CheckDuplicates  bool
}

// FetchResults contains a summary of a fetch operation
// XXX is there a way to return database ids here?
type FetchResults struct {
	NewItems     int
	RequestCount int
	Complete     bool
	Errors       []error
}

// add a new non-fatal error to the result
func (fr *FetchResults) error(e error) {
	fr.Errors = append(fr.Errors, e)
}

func (fr *FetchResults) errorMsg(message string) {
	fr.Errors = append(fr.Errors, errors.New(message))
}

// FetchLatestScrobbles downloads scrobbles for a given user account
// an error being returned means no updates were done, but FetchResults being returned
// doesn't mean that no errors occurred (i.e. the update may be incomplete)
func (this *Fetcher) FetchLatestScrobbles(opts FetchOptions) (FetchResults, error) {
	var err error
	fetchResults := FetchResults{
		Complete: false,
	}

	// returns err on nonexistent/corrupt db, zero val on empty db
	latestDBTime, err := this.db.FindLatestTimestamp()
	if err != nil {
		return fetchResults, err
	}

	// XXX maybe clean up old ticker if it still exists? otherwise the fetcher is
	// single-shot
	this.throttle = time.Tick(time.Duration(opts.APIThrottleDelay) * time.Second)

	/*
		three choices for start state:

		- new database, so everything must be dl'd
		  to: max(uts) from first server response
		  from: nil

		- incremental update (most common case)
		  to: max(uts) from first server response
		  from: max(uts) from local database

		- recover from a checkpoint file
		  all values come from the checkpoint

	*/
	var state traversalState

	if checkpointExists() {
		fmt.Println("resuming from checkpoint file")
		state, err = resumeCheckpoint()
		if err != nil {
			return fetchResults, errors.New("error resuming checkpoint")
		}
		if state.Database != this.db.Path {
			return fetchResults, errors.New("recovering from checkpoint from different database")
		}
	} else if latestDBTime > 0 {
		fmt.Println("doing incremental update")
		fmt.Printf("latest db time:%d [%v]\n", latestDBTime, time.Unix(latestDBTime, 0).UTC()) // XXX
		// use 1 greater than the max time or the latest track will be duplicated
		state = traversalState{
			User:     this.creds.Username,
			Database: this.db.Path,
			From:     latestDBTime + 1,
		}
	} else {
		fmt.Println("doing initial download for new database")
		state = traversalState{
			User:     this.creds.Username,
			Database: this.db.Path,
		}
	}
	fmt.Printf("start state: %+v\n", state)

	errCount := 0 // number of successive errors
	maxRetries := 3

	done := false
	requestLimit := opts.RequestLimit

	for !done {
		newState, tracks, err := getNextTracks(this, state)
		fetchResults.RequestCount++

		if err != nil {
			errCount++
			// XXX use golang 1.13 error wrapping?
			fetchResults.error(err)
			fmt.Println("Error on api call:")
			fmt.Println(err)

			if errCount > maxRetries {
				fmt.Println("Giving up after max retries")
				fetchResults.errorMsg("Giving up after max retries")
				break
			} else {
				backoff := util.Pow(2, errCount+1)
				fmt.Printf("Retrying in %d seconds\n", backoff)
				time.Sleep(time.Duration(backoff) * time.Second)
				continue
			}
		} else {
			errCount = 0
		}

		// XXX review error handling here
		// XXX can StoreActivity return database ids?
		fmt.Printf("* got %d tracks\n", len(tracks))
		err = this.db.StoreActivity(tracks)
		if err != nil {
			fetchResults.error(err)
			fmt.Println("error saving tracks")
			fmt.Println(err)
			break
		}
		fetchResults.NewItems += len(tracks)

		// write checkpoint and update state only if there
		// were no errors processing the items
		fmt.Printf("* new state: %+v\n", newState)
		if !newState.isComplete() {
			writeCheckpoint(checkpointFilename, newState)
			state = newState
		} else {
			// does this mean no more calls, or is stopping here and off-by-one?
			done = true
		}

		// only break from request limit after the checkpoint has
		// been written so it's safe to resume
		if requestLimit > 0 && fetchResults.RequestCount >= requestLimit {
			fetchResults.errorMsg("request limit exceeded, exiting")
			fmt.Println("request limit exceeded, exiting")
			break
		}
	}
	fetchResults.Complete = done

	// completed! so we can remove the checkpoint file (if it exists)
	if checkpointExists() {
		err = os.Remove(checkpointFilename)
		if err != nil {
			// XXX golang 1.13 error wrapping?
			// XXX also, does this count as incomplete?
			fetchResults.error(err)
			fmt.Println("error removing checkpoint file. manually clean this up before next run")
		}
	}
	return fetchResults, nil
}
