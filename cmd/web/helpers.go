package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"text/template"
)

const logStackTraces bool = false

func (app *application) serverError(w http.ResponseWriter, err error) {
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

// "tmpl" => "${tmpl}.page.tmpl"
func renderTemplate(w http.ResponseWriter, tmpl string) {
	// xxx factor this out & clean it up
	// first elt in array is the main template, others are deps
	prefix := "./ui/html/"
	files := []string{
		prefix + tmpl + ".page.tmpl",
		prefix + "base.layout.tmpl",
		prefix + "topnav.partial.tmpl",
		prefix + "datebar.partial.tmpl",
	}
	ts, err := template.ParseFiles(files...)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}

	err = ts.Execute(w, nil)
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
