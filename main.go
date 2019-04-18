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
		epoch, err := strconv.Atoi(t.Date.Uts)
		if err != nil {
			dateSuffix = "[ERR]"
		} else {
			t := time.Unix(int64(epoch), 0)
			dateSuffix = fmt.Sprintf("[%d] %v", epoch, t)
		}
	}
	fmt.Printf("%s - %s %s\n", t.Name, t.Artist.Name, dateSuffix)
}

type traversalState struct {
	User        string
	Page        int
	TotalPages  int
	TotalTracks int
	Anchor      int64
}

// get the next page of responses, given a previous state
func getNextTracks(current traversalState) (traversalState, []TrackInfo, error) {

	tracks := make([]TrackInfo, 0)
	nextState := traversalState{}

	// this lastfm.P thing doesn't seem very typesafe?
	params := lastfm.P{
		"user": "grgbrn",
	}
	// only need to pass the to (anchor) and page params from currentState
	if current.Anchor != 0 {
		params["to"] = current.Anchor
	}
	if current.Page != 0 {
		params["page"] = current.Page
	}
	// fmt.Printf("%+V\n", current)
	// panic("go no further!")

	recentTracks, err := api.User.GetRecentTracks(params)

	if err != nil {
		fmt.Println("error making getRecentTracks request")
		return nextState, tracks, err
	}
	fmt.Printf("got page %d/%d\n", recentTracks.Page, recentTracks.TotalPages)

	// preserve user & anchor, update the rest from the response
	nextState.User = current.User
	nextState.Page = recentTracks.Page
	nextState.TotalPages = recentTracks.TotalPages
	nextState.TotalTracks = recentTracks.Total
	if current.Anchor != 0 {
		nextState.Anchor = current.Anchor
	} else {
		// must be initial call, so have to find the anchor

		// XXX how to handle an unparseable uts?
		// XXX and can we assume that it's always going to be the first not-playing element?
		// XXX may need to skip currently playing track too
		var maxUTS int64
		for _, track := range recentTracks.Tracks {
			tmp, _ := getParsedUTS(track)
			if tmp > maxUTS {
				maxUTS = tmp
			}
		}
		nextState.Anchor = maxUTS
	}

	// can't return recentTracks.Tracks as a []TrackInfo for some reason
	// but building up another identical list seems to work
	for _, track := range recentTracks.Tracks {
		tracks = append(tracks, track)
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

	// XXX figure out where to get initial state from... if there's a checkpoint
	// file that needs to be reloaded, otherwise find the max id from the database?
	// and if there's no database just create a blank with only the user set?
	initialState := traversalState{
		User: "grgbrn",
		//Anchor: 1555586216,
	}

	// XXX while what? how do i determine when i'm done? no new tracks returned?
	// can the struct itself tell me? Page == TotalPages? would only work for the first

	fmt.Printf("initial state: %+v\n", initialState)

	newState, tracks, err := getNextTracks(initialState)
	if err != nil {
		panic("error on first call")
	}

	fmt.Printf("got %d tracks\n", len(tracks))
	fmt.Printf("state after initial call: %+v\n", newState)

	for _, t := range tracks {
		printTrack(t)
	}
}
