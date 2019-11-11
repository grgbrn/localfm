package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"text/template"

	"bitbucket.org/grgbrn/localfm/pkg/util"
	"golang.org/x/crypto/bcrypt"
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
func renderTemplate(w http.ResponseWriter, tmpl string, td *templateData) {
	// xxx factor this out & clean it up
	// first elt in array is the main template, others are deps
	fileRoot := util.GetEnvStr("STATIC_FILE_ROOT", ".")
	prefix := path.Join(fileRoot, "ui/html/")
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	files := []string{
		prefix + tmpl + ".page.tmpl",
		prefix + "base.layout.tmpl",
		prefix + "topnav.partial.tmpl",
		prefix + "datebar.partial.tmpl",
		prefix + "nextbar.partial.tmpl",
	}
	ts, err := template.ParseFiles(files...)
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

// checks if email/passwd are valid login credentials
// returns (userId, nil) on successful login, where userid > 0
// returns (-1,nil) if credentials don't match
// err is only returned for internal errors
func authenticateUser(email, passwd string) (int, error) {

	// single user's login credentials are in env vars
	// passwd must be a bcrypt hashed string, 60 chars long
	// generate one with:
	//
	// hash, err := bcrypt.GenerateFromPassword([]byte("my plain text password"), 12)
	//
	// or:
	//
	// htpasswd -bnBC 12 "" password | tr -d ':\n'
	const userID = 100
	envLogin := os.Getenv("LOGIN_USER")
	if envLogin == "" {
		panic("Must set LOGIN_USER environment var")
	}
	envPass := os.Getenv("LOGIN_PASSWD")
	if envLogin == "" {
		panic("Must set LOGIN_PASSWD environment var")
	}

	if envLogin != email {
		return -1, nil
	}

	hashedPassword := []byte(envPass)

	// Check whether the hashed password and plain-text password provided match
	err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(passwd))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return -1, nil
		}
		return 0, err
	}
	// Otherwise, the password is correct. Return the user ID.
	return userID, nil
}

func (app *Application) isAuthenticated(r *http.Request) bool {
	return app.session.Exists(r, "authenticatedUserID")
}
