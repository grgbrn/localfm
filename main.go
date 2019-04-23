package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/shkh/lastfm-go/lastfm"
)

const checkpointFilename string = "checkpoint.json"
const apiThrottleSecs int64 = 2

// module globals
var api *lastfm.Api
var throttle <-chan time.Time

// lastfm lib doesn't define useful sub-structs for it's
// result types, so do it myself...
// XML annotations aren't necessary but required for types to match
type TrackInfo struct {
	NowPlaying string `xml:"nowplaying,attr,omitempty"`
	Artist     struct {
		Name string `xml:",chardata"`
		Mbid string `xml:"mbid,attr"`
	} `xml:"artist"`
	Name       string `xml:"name"`
	Streamable string `xml:"streamable"`
	Mbid       string `xml:"mbid"`
	Album      struct {
		Name string `xml:",chardata"`
		Mbid string `xml:"mbid,attr"`
	} `xml:"album"`
	Url    string `xml:"url"`
	Images []struct {
		Size string `xml:"size,attr"`
		Url  string `xml:",chardata"`
	} `xml:"image"`
	Date struct {
		Uts  string `xml:"uts,attr"`
		Date string `xml:",chardata"`
	} `xml:"date"`
}

func getParsedUTS(ti TrackInfo) (int64, error) {
	epoch, err := strconv.Atoi(ti.Date.Uts)
	if err != nil {
		return 0, err
	}
	return int64(epoch), nil
}

func printTrack(t TrackInfo) {
	var dateSuffix string
	if t.NowPlaying == "true" {
		dateSuffix = "(current)"
	} else {
		epoch, err := getParsedUTS(t)
		if err != nil {
			dateSuffix = "[ERR]"
		} else {
			// tmp := time.Unix(epoch, 0)
			dateSuffix = fmt.Sprintf("[%d] %s", epoch, t.Date.Date)
		}
	}
	fmt.Printf("%s - %s %s\n", t.Name, t.Artist.Name, dateSuffix)
}

type traversalState struct {
	User string

	Page        int
	TotalPages  int
	TotalTracks int

	From int64
	To   int64 // nee Anchor
}

func (ts traversalState) isComplete() bool {
	return ts.TotalPages > 0 && ts.Page == ts.TotalPages
}

// processResponse finds the max uts in a response, filters out the "now playing"
// track (if any) and coerces remote API responses into local TrackInfo interface
func processResponse(recentTracks lastfm.UserGetRecentTracks) (int64, []TrackInfo) {
	var maxUTS int64
	tracks := make([]TrackInfo, 0)

	for _, track := range recentTracks.Tracks {
		if track.NowPlaying != "true" {
			tmp, _ := getParsedUTS(track)
			if tmp > maxUTS {
				maxUTS = tmp
			}
			tracks = append(tracks, track)
		}
	}

	return maxUTS, tracks
}

func ts(t time.Time) string {
	return t.Format("15:04:05.00000")
}

// get the next page of responses, given a previous state
func getNextTracks(current traversalState) (traversalState, []TrackInfo, error) {

	if current.isComplete() {
		panic("getNextTracks called on a completed state")
	}

	tracks := make([]TrackInfo, 0) // XXX any other way to define this without make()???
	nextState := traversalState{}

	// this lastfm.P thing doesn't seem very typesafe?
	params := lastfm.P{
		"user":  "grgbrn",
		"limit": 10, // XXX only for debugging
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

	maxUTS, tracks := processResponse(recentTracks)

	// preserve user & anchor, update the rest from the response
	nextState.User = current.User
	nextState.Page = recentTracks.Page + 1
	nextState.TotalPages = recentTracks.TotalPages
	nextState.TotalTracks = recentTracks.Total
	nextState.From = current.From
	if current.To != 0 {
		nextState.To = current.To
	} else {
		// must be initial call, so need to use the maxUTS from the response
		nextState.To = maxUTS
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
	_, err := os.Stat(checkpointFilename)
	return !os.IsNotExist(err)
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

func main() {
	APIKey := os.Getenv("LASTFM_API_KEY")
	APISecret := os.Getenv("LASTFM_API_SECRET")

	if APIKey == "" || APISecret == "" {
		panic("Must set LASTFM_API_KEY and LASTFM_API_SECRET environment vars")
	}

	api = lastfm.New(APIKey, APISecret)
	throttle = time.Tick(time.Duration(apiThrottleSecs) * time.Second)

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
	var err error
	var state traversalState

	if checkpointExists() {
		fmt.Println("resuming from checkpoint file")
		state, err = resumeCheckpoint()
		if err != nil {
			panic("error resuming checkpoint")
		}
	} else {
		state = traversalState{
			User: "grgbrn",
		}
	}
	// XXX check database for incremental update
	fmt.Printf("initial state: %+v\n", state)

	requestLimit := 3
	requestCount := 0

	done := false

	for !done {
		newState, tracks, err := getNextTracks(state)
		requestCount++

		if err != nil {
			// XXX some kind of exponential backoff and continue
			panic("error on API call")
		}

		fmt.Printf("* got %d tracks\n", len(tracks))

		for _, t := range tracks {
			printTrack(t)
		}
		// xxx break if there's an error processing the items

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
		if requestCount >= requestLimit {
			fmt.Println("request limit exceeded, exiting!")
			break
		}
	}

	// completed! so we can remove the checkpoint file
	// XXX also print some stats here
	if done {
		err = os.Remove(checkpointFilename)
		if err != nil {
			fmt.Println("error removing checkpoint file. manually clean this up before next run")
			// XXX return an error code here
		}
	} else {
		fmt.Println("incomplete run for some reason! probably need to continue from checkpoint")
	}
}
