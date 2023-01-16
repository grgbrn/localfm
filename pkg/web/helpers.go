package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"bitbucket.org/grgbrn/localfm/pkg/util"
)

const logStackTraces bool = false

func (app *Application) serverError(w http.ResponseWriter, err error) {
	if logStackTraces {
		// from "let's go" ch3.04
		// XXX but this could use some tweaking
		trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
		//app.err.Println(trace)
		app.err.Output(2, trace)
	} else {
		app.err.Println(err.Error())
	}

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// XXX maybe add a clientError also?
// file:///home/greg/Downloads/lets-go/html/03.04-centralized-error-handling.html

type templateData struct {
	Error string
}

// "tmpl" => "${tmpl}.page.tmpl"
func renderTemplate(w http.ResponseWriter, tmpl string, td interface{}) {
	// xxx factor this out & clean it up
	// first elt in array is the main template, others are deps
	fileRoot := util.GetEnvStr("STATIC_FILE_ROOT", ".")
	prefix := path.Join(fileRoot, "ui/html/")
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	files := []string{
		//prefix + tmpl + ".page.tmpl",
		prefix + tmpl,
		prefix + "recent.tmpl",
		prefix + "tracks.tmpl",
		prefix + "artists.tmpl",
		prefix + "base.layout.tmpl",
		prefix + "topnav.partial.tmpl",
		prefix + "datebar.partial.tmpl",
		prefix + "nextbar.partial.tmpl",
	}

	var functions = template.FuncMap{
		"dateLabel":  dateLabel,
		"prettyTime": prettyTime,
	}

	ts, err := template.New(tmpl).Funcs(functions).ParseFiles(files...)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}

	err = ts.Execute(w, td)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
	}
}

func renderJSON(w http.ResponseWriter, status int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	w.Write([]byte(response))
}

// template formatters

// returns "today", "yesterday", or a date string
func dateLabel(t time.Time) string {
	now := time.Now()
	if now.Year() == t.Year() && now.YearDay() == t.YearDay() {
		return "Today"
	} else {
		yesterday := now.AddDate(0, 0, -1)
		if yesterday.Year() == t.Year() && yesterday.YearDay() == t.YearDay() {
			return "Yesterday"
		}
	}
	return t.Format("Mon Jan 2 2006")
}

// returns a relative time if in the last day, otherwise "kitchen" time
func prettyTime(t time.Time) string {
	diff := time.Now().Unix() - t.Unix()
	dayDiff := int(math.Floor(float64(diff) / 86400))

	if dayDiff == 0 {
		switch {
		case diff < 60:
			return "just now"
		case diff < 120:
			return "1 minute ago"
		case diff < 3600:
			return fmt.Sprintf("%d minutes ago", diff/60)
		case diff < 7200:
			return "1 hour ago"
		case diff < 86400:
			return fmt.Sprintf("%d hours ago", diff/3600)
		}
	}

	return t.Format(time.Kitchen)
}
