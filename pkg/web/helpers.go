package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
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

type errorTemplateData struct {
	Error string
}

func (app *Application) renderTemplate(w http.ResponseWriter, tmpl string, td interface{}) {

	ts, ok := app.templateCache[tmpl]
	if !ok {
		err := fmt.Errorf("Template %s does not exist", tmpl)
		app.serverError(w, err)
		return
	}

	buf := new(bytes.Buffer)

	err := ts.ExecuteTemplate(buf, tmpl, td)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
	}

	w.WriteHeader(http.StatusOK)
	buf.WriteTo(w)
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
