package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/util"

	"github.com/shkh/lastfm-go/lastfm"
)

const checkpointFilename string = "checkpoint.json"

// module globals
var api *lastfm.Api
var throttle <-chan time.Time

type traversalState struct {
	User     string
	Database string

	Page        int
	TotalPages  int
	TotalTracks int

	From int64
	To   int64 // nee Anchor
}

func (ts traversalState) isInitial() bool {
	return ts.To == 0
}

func (ts traversalState) isComplete() bool {
	return !ts.isInitial() && (ts.TotalPages == 0 || ts.Page > ts.TotalPages)
}

// processResponse finds the max uts in a response, filters out the "now playing"
// track (if any) and coerces remote API responses into local TrackInfo interface
func processResponse(recentTracks lastfm.UserGetRecentTracks) (int64, []m.TrackInfo) {
	var maxUTS int64
	tracks := make([]m.TrackInfo, 0)

	for _, track := range recentTracks.Tracks {
		if track.NowPlaying != "true" {
			tmp, _ := m.GetParsedUTS(track)
			if tmp > maxUTS {
				maxUTS = tmp
			}
			tracks = append(tracks, track)
		}
	}

	return maxUTS, tracks
}

// get the next page of responses, given a previous state
func getNextTracks(current traversalState) (traversalState, []m.TrackInfo, error) {

	if current.isComplete() {
		panic("getNextTracks called on a completed state")
	}

	tracks := []m.TrackInfo{}
	nextState := traversalState{
		User:     current.User,
		Database: current.Database,
	}

	// this lastfm.P thing doesn't seem very typesafe?
	params := lastfm.P{
		"user": current.User,
	}
	// need to pass to, from, page params from current state
	if current.To != 0 {
		params["to"] = current.To
	}
	if current.From != 0 {
		params["from"] = current.From
	}
	if current.Page != 0 {
		params["page"] = current.Page
	}

	// blocking read from rate-limit channel
	<-throttle
	fmt.Printf("== calling GetRecentTracks %+v\n", params)
	fmt.Printf("== [%s]\n", time.Now())
	recentTracks, err := api.User.GetRecentTracks(params)

	if err != nil {
		return nextState, tracks, err
	}
	fmt.Printf("got page %d/%d\n", recentTracks.Page, recentTracks.TotalPages)

	// update the next state with totals from the response
	// (these should not change during a traversal)
	nextState.TotalPages = recentTracks.TotalPages
	nextState.TotalTracks = recentTracks.Total

	maxUTS, tracks := processResponse(recentTracks)

	nextState.Page = recentTracks.Page + 1
	nextState.From = current.From
	if current.To != 0 {
		nextState.To = current.To
	} else {
		// must be initial call, so need to find maxUTS from the response
		// use 1 greater than the maxUTS or the first track will be excluded
		nextState.To = maxUTS + 1
	}

	return nextState, tracks, nil
}

func writeCheckpoint(path string, state traversalState) error {
	jout, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, jout, 0644)
	if err != nil {
		return err
	}
	return nil
}

func checkpointExists() bool {
	return util.FileExists(checkpointFilename)
}

func resumeCheckpoint() (traversalState, error) {
	newState := traversalState{}

	dat, err := ioutil.ReadFile(checkpointFilename)
	if err != nil {
		return newState, err
	}

	if err := json.Unmarshal(dat, &newState); err != nil {
		return newState, err
	}
	return newState, nil
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
func FetchLatestScrobbles(db *m.Database, creds LastFMCredentials, opts FetchOptions) (FetchResults, error) {
	var err error
	fetchResults := FetchResults{
		Complete: false,
	}

	// returns err on nonexistent/corrupt db, zero val on empty db
	latestDBTime, err := db.FindLatestTimestamp()
	if err != nil {
		return fetchResults, err
	}

	api = lastfm.New(creds.APIKey, creds.APISecret)
	throttle = time.Tick(time.Duration(opts.APIThrottleDelay) * time.Second)

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
		if state.Database != db.Path {
			return fetchResults, errors.New("recovering from checkpoint from different database")
		}
	} else if latestDBTime > 0 {
		fmt.Println("doing incremental update")
		fmt.Printf("latest db time:%d [%v]\n", latestDBTime, time.Unix(latestDBTime, 0).UTC()) // XXX
		// use 1 greater than the max time or the latest track will be duplicated
		state = traversalState{
			User:     creds.Username,
			Database: db.Path,
			From:     latestDBTime + 1,
		}
	} else {
		fmt.Println("doing initial download for new database")
		state = traversalState{
			User:     creds.Username,
			Database: db.Path,
		}
	}
	fmt.Printf("start state: %+v\n", state)

	errCount := 0 // number of successive errors
	maxRetries := 3

	done := false
	requestLimit := opts.RequestLimit

	for !done {
		newState, tracks, err := getNextTracks(state)
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
		err = db.StoreActivity(tracks)
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
