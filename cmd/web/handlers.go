package main

import (
	"html/template"
	"log"
	"net/http"
)

func renderTemplate(w http.ResponseWriter, tmpl string) {
	templatePath := "./ui/html/" + tmpl + ".html"
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}

	err = t.Execute(w, nil)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, "./recent", http.StatusTemporaryRedirect)
}

func recent(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "recent")
}

func monthly(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "monthly")
}

func artists(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "artists")
}
