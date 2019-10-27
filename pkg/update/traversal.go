package update

import (
	"fmt"
	"time"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"github.com/shkh/lastfm-go/lastfm"
)

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
func getNextTracks(fetcher *Fetcher, current traversalState) (traversalState, []m.TrackInfo, error) {

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
	<-fetcher.throttle
	fmt.Printf("== calling GetRecentTracks %+v\n", params)
	fmt.Printf("== [%s]\n", time.Now())
	recentTracks, err := fetcher.lastfm.User.GetRecentTracks(params)

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
