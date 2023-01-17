package web

import (
	"fmt"
	"html/template"
	"math"
	"path/filepath"
	"time"
)

func newTemplateCache() (map[string]*template.Template, error) {

	cache := map[string]*template.Template{}

	// get all page-level templates that can be rendered directly
	pages, err := filepath.Glob("./ui/html/*.tmpl")
	if err != nil {
		return nil, err
	}

	// get all partials and base layouts that are needed as dependencies
	partials, err := filepath.Glob("./ui/html/partial/*.tmpl")
	if err != nil {
		return nil, err
	}

	// custom formatting functions
	var functions = template.FuncMap{
		"dateLabel":  dateLabel,
		"prettyTime": prettyTime,
	}

	for _, page := range pages {
		name := filepath.Base(page)

		// make a copy of partials
		files := make([]string, len(partials))
		copy(files, partials)
		files = append(files, page)

		ts, err := template.New(name).Funcs(functions).ParseFiles(files...)
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	return cache, nil
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
