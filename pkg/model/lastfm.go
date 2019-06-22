package model

import (
	"fmt"
	"strconv"
	"time"
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

// XXX maybe this is overkill, just choose the last one for now
var trackInfoImageWeights = map[string]int{
	"small":      1,
	"medium":     2,
	"large":      3,
	"extralarge": 4,
}

// GetParsedUTS returns the epoch timestamp from this TrackInfo, or an error
func GetParsedUTS(ti TrackInfo) (int64, error) {
	epoch, err := strconv.Atoi(ti.Date.Uts)
	if err != nil {
		return 0, err
	}
	return int64(epoch), nil
}

// GetParsedTime returns a time.Time object in UTC
func GetParsedTime(ti TrackInfo) (time.Time, error) {
	var t time.Time
	uts, err := GetParsedUTS(ti)
	if err != nil {
		return t, err
	}
	return time.Unix(uts, 0).UTC(), nil
}

func printTrack(t TrackInfo) {
	var dateSuffix string
	if t.NowPlaying == "true" {
		dateSuffix = "(current)"
	} else {
		epoch, err := GetParsedUTS(t)
		if err != nil {
			dateSuffix = "[ERR]"
		} else {
			// tmp := time.Unix(epoch, 0)
			dateSuffix = fmt.Sprintf("[%d] %s", epoch, t.Date.Date)
		}
	}
	fmt.Printf("%s - %s %s\n", t.Name, t.Artist.Name, dateSuffix)
}

// ChooseImageURL selects the best quality image url from a list of choices in a TrackInfo
func ChooseImageURL(t TrackInfo) string {

	var best int
	var url string

	for _, s := range t.Images {
		v, _ := trackInfoImageWeights[s.Size]
		if v > best {
			best = v
			url = s.Url
		}
	}
	return url
}
