package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"text/template"
	"wargh/db"
)

var templates *template.Template

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		//templates.Lookup("index.html").Execute(w, nil)
		redirectError(w, r, ERROR_TEST)
	}
}

const (
	ERROR_UNKNOWN = 0
	ERROR_TEST    = 1
)

var ERROR_TEXT = []string{
	"Unknown error",
	"Test error",
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	errorParam := r.URL.Query().Get("error")
	var errorCode int
	if errorParam != "" {
		var err error
		errorCode, err = strconv.Atoi(errorParam)
		if err != nil {
			errorCode = 0
		}
	} else {
		errorCode = 0
	}

	if errorCode < 0 || errorCode >= len(ERROR_TEXT) {
		errorCode = 0
	}

	templates.Lookup("error.html").Execute(w, ERROR_TEXT[errorCode])
}

func redirectError(w http.ResponseWriter, r *http.Request, errorCode int) {
	http.Redirect(w, r, fmt.Sprintf("/error?error=%d", errorCode), http.StatusSeeOther)
}

func main() {
	db.Init(&db.DBConfig{
		DBPath:         "wargh.db",
		MigrationsPath: "migrations",
	})

	DB, err := db.Open()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	defer DB.Close()

	templates, err = template.New("templates").ParseFiles(
		"templates/index.html",
		"templates/error.html",
	)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/error", errorHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
