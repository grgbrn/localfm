package main

import (
	"log"
	"net/http"
)

func main() {
	// Use the http.NewServeMux() function to initialize a new servemux, then
	// register the home function as the handler for the "/" URL pattern.
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)

	mux.HandleFunc("/recent", recentPage)
	mux.HandleFunc("/monthly", monthlyPage)
	mux.HandleFunc("/artists", artistsPage)

	mux.HandleFunc("/data/artists", artistsData)
	mux.HandleFunc("/data/monthlyArtists", monthlyArtistData)
	mux.HandleFunc("/data/monthlyTracks", monthlyTrackData)

	// set up static file server to ignore /ui/static/ prefix
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	log.Println("Starting server on :4000")
	err := http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}
