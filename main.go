package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/shkh/lastfm-go/lastfm"
)

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

func main() {
	APIKey := os.Getenv("LASTFM_API_KEY")
	APISecret := os.Getenv("LASTFM_API_SECRET")

	if APIKey == "" || APISecret == "" {
		panic("Must set LASTFM_API_KEY and LASTFM_API_SECRET environment vars")
	}

	api := lastfm.New(APIKey, APISecret)

	// this lastfm.P thing seems like a pretty awkward way to pass params
	recentTracks, err := api.User.GetRecentTracks(lastfm.P{"user": "grgbrn"})

	if err != nil {
		fmt.Println("error making getRecentTracks request")
		fmt.Println(err)
	}

	fmt.Printf("got page %d/%d\n", recentTracks.Page, recentTracks.TotalPages)

	for _, track := range recentTracks.Tracks {
		printTrack(track)
	}
}
