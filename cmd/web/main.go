package main

import (
	"crypto/rand"
	"log"
	"net/http"
	"os"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/web"
)


func main() {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	//
	// database init
	//
	DSN := os.Getenv("DSN")
	if DSN == "" {
		panic("Must set DSN environment var")
	}
	db, err := m.Open(DSN)
	if err != nil {
		panic(err)
	}

	// init session store
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		infoLog.Println("SESSION_SECRET not set, using temporary value")

		key := [32]byte{}
		_, err := rand.Read(key[:])
		if err != nil {
			panic(err) // XXX
		}
		sessionSecret = string(key[:])
	}
	if len(sessionSecret) != 32 {
		panic("SESSION_SECRET must contain 32 bytes")
	}

	app, err := web.CreateApp(db, sessionSecret, infoLog, errorLog)
	if err != nil {
		panic(err)
	}

	addr := ":4000" // XXX
	srv := &http.Server{
		Addr:     addr,
		ErrorLog: errorLog,
		Handler:  app.Mux,
	}

	infoLog.Printf("Starting server on %s\n", addr)
	err = srv.ListenAndServe()
	errorLog.Fatal(err)
}
