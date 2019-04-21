package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/shkh/lastfm-go/lastfm"
)

// module globals
var api *lastfm.Api

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
			dateSuffix = fmt.Sprintf("[%d]", epoch)
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

// get the next page of responses, given a previous state
func getNextTracks(current traversalState) (traversalState, []TrackInfo, error) {

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
	fmt.Printf("== calling GetRecentTracks %+v\n", params)

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

func main() {
	APIKey := os.Getenv("LASTFM_API_KEY")
	APISecret := os.Getenv("LASTFM_API_SECRET")

	if APIKey == "" || APISecret == "" {
		panic("Must set LASTFM_API_KEY and LASTFM_API_SECRET environment vars")
	}

	api = lastfm.New(APIKey, APISecret)

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
	state := traversalState{
		User: "grgbrn",
		//From: 1555427528,
	}
	fmt.Printf("initial state: %+v\n", state)

	// simple test to just make 3 repeated calls
	c := 0
	for c < 3 {
		newState, tracks, err := getNextTracks(state)
		c++
		if err != nil {
			panic("error on API call")
		}

		fmt.Printf("* new state: %+v\n", newState)
		fmt.Printf("* got %d tracks\n", len(tracks))
		state = newState

		for _, t := range tracks {
			printTrack(t)
		}

		time.Sleep(2 * time.Second)
	}

}
