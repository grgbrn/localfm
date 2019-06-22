package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
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

type artistResult struct {
	Name      string   `json:"artist"`
	Count     int      `json:"count"`
	ImageURLs []string `json:"urls"`
}

// json data handlers
func (app *application) artistsData(w http.ResponseWriter, r *http.Request) {

	query := `select a.artist, count(*) as c, group_concat(distinct i.url)
	from activity a
	join image i on a.image_id = i.id
	where a.dt >= ? and a.dt < ?
	group by a.artist
	order by c desc limit ?;`

	// XXX need correct parameters here
	start := "2019-06-01"
	end := "2019-07-01"
	lim := 20

	rows, err := app.db.Query(query, start, end, lim)
	if err != nil {
		panic("handle an error") // XXX
	}
	defer rows.Close()

	var artists []artistResult

	for rows.Next() {
		groupConcat := ""
		res := artistResult{}

		err = rows.Scan(&res.Name, &res.Count, &groupConcat)
		if err != nil {
			panic("handle an error") // XXX
		}

		res.ImageURLs = strings.Split(groupConcat, ",")
		artists = append(artists, res)
	}

	renderJSON(w, http.StatusOK, artists)
}

func (app *application) monthlyArtistData(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./ui/static/data/monthly_artists.json")
}

func (app *application) monthlyTrackData(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./ui/static/data/monthly_track.json")
}
