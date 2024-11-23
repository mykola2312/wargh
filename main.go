package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"text/template"
	"wargh/db"

	"golang.org/x/crypto/bcrypt"
)

var DB *sql.DB
var templates *template.Template

type UserSession struct {
	Id int
}

var sessions map[string]UserSession

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		templates.Lookup("index.html").Execute(w, nil)
	}
}

const (
	ERROR_UNKNOWN                = 0
	ERROR_INVALID_INPUT          = 1
	ERROR_INVALID_LOGIN_PASSWORD = 2
	ERROR_UNAUTHORIZED           = 3
)

var ERROR_TEXT = []string{
	"Unknown error",
	"Invalid input",
	"Invalid login or password",
	"Unathorized",
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

func isFirstUser() (bool, error) {
	row := DB.QueryRow("SELECT COUNT(id) FROM `user`;")
	if row.Err() != nil {
		return false, row.Err()
	}

	var count int
	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

func createUser(login string, password string) (int, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return 0, err
	}

	_, err = DB.Exec("INSERT INTO `user` (login, password) VALUES (?,?);", login, bytes)
	if err != nil {
		return 0, err
	}

	row := DB.QueryRow("SELECT id FROM `user` WHERE login = ?;", login)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var id int
	row.Scan(&id)

	return id, nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		templates.Lookup("login.html").Execute(w, nil)
	} else if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Print(err)

			redirectError(w, r, ERROR_UNKNOWN)
			return
		}

		loginParam := r.Form.Get("login")
		passwordParam := r.Form.Get("password")
		if loginParam == "" || passwordParam == "" {
			redirectError(w, r, ERROR_INVALID_INPUT)
			return
		}

		isFirst, err := isFirstUser()
		if err != nil {
			log.Print(err)

			redirectError(w, r, ERROR_UNKNOWN)
		}

		if isFirst {
			id, err := createUser(loginParam, passwordParam)
			if err != nil {
				log.Print(err)
			} else {
				log.Printf("created user %d\n", id)
			}

			// create session
		} else {
			// try authorize
			row := DB.QueryRow("SELECT id,password FROM `user` WHERE login = ?;", loginParam)
			if row.Err() != nil {
				if row.Err().Error() == sql.ErrNoRows.Error() {
					redirectError(w, r, ERROR_INVALID_LOGIN_PASSWORD)
					return
				} else {
					log.Print(err)

					redirectError(w, r, ERROR_UNKNOWN)
					return
				}
			}

			var id int
			password := make([]byte, 8)

			err = row.Scan(&id, &password)
			if err != nil {
				// can't scan blob? fatal error
				log.Fatal(err)
			}

			if bcrypt.CompareHashAndPassword(password, []byte(passwordParam)) != nil {
				redirectError(w, r, ERROR_INVALID_LOGIN_PASSWORD)
				return
			}

			// create session
			log.Printf("user %d authorized!\n", id)
		}
	}
}

func main() {
	db.Init(&db.DBConfig{
		DBPath:         "wargh.db",
		MigrationsPath: "migrations",
	})

	var err error
	DB, err = db.Open()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	defer DB.Close()

	templates, err = template.New("templates").ParseFiles(
		"templates/index.html",
		"templates/error.html",
		"templates/login.html",
	)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/error", errorHandler)
	http.HandleFunc("/login", loginHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
