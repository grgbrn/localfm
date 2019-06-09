package localfm

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shkh/lastfm-go/lastfm"
)

const checkpointFilename string = "checkpoint.json"

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

// XXX maybe this is overkill, just choose the last one for now
var trackInfoImageWeights = map[string]int{
	"small":      1,
	"medium":     2,
	"large":      3,
	"extralarge": 4,
}

func getParsedUTS(ti TrackInfo) (int64, error) {
	epoch, err := strconv.Atoi(ti.Date.Uts)
	if err != nil {
		return 0, err
	}
	return int64(epoch), nil
}

// getParsedTime returns a time.Time object in UTC
func getParsedTime(ti TrackInfo) (time.Time, error) {
	var t time.Time
	uts, err := getParsedUTS(ti)
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

	if current.isComplete() {
		panic("getNextTracks called on a completed state")
	}

	tracks := []TrackInfo{}
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func checkpointExists() bool {
	return fileExists(checkpointFilename)
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

func Main(apiThrottleDelay int, requestLimit int, checkAllDuplicates bool) {
	var err error

	//
	// initialize database
	//
	var db *sql.DB

	DSN := os.Getenv("DSN")
	if DSN == "" {
		panic("Must set DSN environment var")
	}
	// mimic DSN format from earlier python version of this tool
	// "sqlite:///foo.db"
	if !strings.HasPrefix(DSN, "sqlite://") {
		panic("DSN var must be of the format 'sqlite:///foo.db'")
	}
	dbPath := DSN[9:]

	// sqlite database drivers will automatically create empty databases
	// if the file doesn't exist, so stat the file first and abort
	// if there's no database (must be manually created with schema)
	if !fileExists(dbPath) {
		panic("Can't open database [0]")
	}

	// this seemingly never returns an error
	db, err = InitDB(dbPath)
	if err != nil {
		panic("Can't open database [1]")
	}

	// returns err on nonexistent/corrupt db, zero val on empty db
	latestDBTime, err := FindLatestTimestamp(db)
	if err != nil {
		panic("Can't open database [2]")
	}

	//
	// initialize lastfm api client
	//
	APIKey := os.Getenv("LASTFM_API_KEY")
	APISecret := os.Getenv("LASTFM_API_SECRET")
	Username := os.Getenv("LASTFM_USERNAME")

	if APIKey == "" || APISecret == "" || Username == "" {
		panic("Must set LASTFM_USERNAME, LASTFM_API_KEY and LASTFM_API_SECRET environment vars")
	}

	api = lastfm.New(APIKey, APISecret)
	throttle = time.Tick(time.Duration(apiThrottleDelay) * time.Second)

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
			panic("error resuming checkpoint")
		}
		if state.Database != dbPath {
			panic("recovering from checkpoint from different database")
		}
	} else if latestDBTime > 0 {
		fmt.Println("doing incremental update")
		fmt.Printf("latest db time:%d [%v]\n", latestDBTime, time.Unix(latestDBTime, 0).UTC()) // XXX
		// use 1 greater than the max time or the latest track will be duplicated
		state = traversalState{
			User:     Username,
			Database: dbPath,
			From:     latestDBTime + 1,
		}
	} else {
		fmt.Println("doing initial download for new database")
		state = traversalState{
			User:     Username,
			Database: dbPath,
		}
	}
	fmt.Printf("start state: %+v\n", state)

	// will not exceed requestLimit param if set (!=0)
	requestCount := 0

	errCount := 0 // number of successive errors
	maxRetries := 3

	newItems := 0

	done := false

	for !done {
		newState, tracks, err := getNextTracks(state)
		requestCount++

		if err != nil {
			errCount++
			fmt.Println("Error on api call:")
			fmt.Println(err)

			if errCount > maxRetries {
				fmt.Println("Giving up after max retries")
				break
			} else {
				backoff := Pow(2, errCount+1)
				fmt.Printf("Retrying in %d seconds\n", backoff)
				time.Sleep(time.Duration(backoff) * time.Second)
				continue
			}
		} else {
			errCount = 0
		}

		// XXX review error handling here
		fmt.Printf("* got %d tracks\n", len(tracks))
		err = StoreActivity(db, tracks)
		if err != nil {
			fmt.Println("error saving tracks!")
			fmt.Println(err)
			break
		}
		newItems += len(tracks)

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
		if requestLimit > 0 && requestCount >= requestLimit {
			fmt.Println("request limit exceeded, exiting!")
			break
		}
	}

	// incremental duplicate suppression if env var is set
	// commandline flag will cause it to re-check the entire database
	duplicateThreshold := os.Getenv("LOCALFM_DUPLICATE_THRESHOLD")
	if checkAllDuplicates && duplicateThreshold == "" {
		fmt.Println("Warning! Must set LOCALFM_DUPLICATE_THRESHOLD with -duplicates flag")
	}
	if duplicateThreshold != "" {
		duplicateThresholdInt, err := strconv.Atoi(duplicateThreshold)
		if err != nil {
			fmt.Printf("Warning! LOCALFM_DUPLICATE_THRESHOLD couldn't be parsed: %v\n", err)
		} else {
			var since int64
			if checkAllDuplicates {
				since = 0
			} else {
				since = latestDBTime
			}

			_, err = FlagDuplicates(db, since, int64(duplicateThresholdInt))
			if err != nil {
				fmt.Printf("Warning! problem flagging duplicates: %v\n", err)
			}
		}
	}

	// completed! so we can remove the checkpoint file (if it exists)
	// XXX also print some stats here
	if done {
		if checkpointExists() {
			err = os.Remove(checkpointFilename)
			if err != nil {
				fmt.Println("error removing checkpoint file. manually clean this up before next run")
				// XXX return an error code here
			}
		}
	} else {
		fmt.Println("incomplete run for some reason! probably need to continue from checkpoint")
	}
}
