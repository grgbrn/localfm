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

// primary html templates
func recentPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "recent")
}

func monthlyPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "monthly")
}

func artistsPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "artists")
}

// json data handlers
func artistsData(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./ui/static/data/artists.json")
}

func monthlyArtistData(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./ui/static/data/monthly_artists.json")
}

func monthlyTrackData(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./ui/static/data/monthly_track.json")
}
