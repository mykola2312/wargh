package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"text/template"
	"time"
	"wargh/db"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

var DB *sql.DB
var templates *template.Template

const COOKIE_NAME = "WARGH_SESSION"
const COOKIE_EXPIRATION = time.Hour * 24 * 7 // 7 days

type UserSession struct {
	Id int
}

var sessions map[string]UserSession = make(map[string]UserSession)

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

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Job struct {
	Command    *exec.Cmd
	StdoutPipe io.ReadCloser
	StderrPipe io.ReadCloser
	Socket     []*websocket.Conn
}

var jobs map[string]Job = make(map[string]Job)

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

func redirectIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

func randString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

func createSession(w http.ResponseWriter, userId int) {
	sessionStr := randString(64)
	sessions[sessionStr] = UserSession{Id: userId}

	http.SetCookie(w, &http.Cookie{
		Name:     COOKIE_NAME,
		Value:    sessionStr,
		Expires:  time.Now().Add(COOKIE_EXPIRATION),
		HttpOnly: true,
	})
}

func checkSession(w http.ResponseWriter, r *http.Request) bool {
	var sessionCookie *http.Cookie = nil
	for _, cookie := range r.Cookies() {
		if cookie.Name == COOKIE_NAME {
			sessionCookie = cookie
		}
	}

	if sessionCookie.Valid() != nil {
		redirectError(w, r, ERROR_UNAUTHORIZED)
		return false
	}

	if _, ok := sessions[sessionCookie.Value]; ok {
		return true
	} else {
		redirectError(w, r, ERROR_UNAUTHORIZED)
		return false
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		templates.Lookup("index.html").Execute(w, nil)
	}
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

			createSession(w, id)
			redirectIndex(w, r)
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

			createSession(w, id)
			redirectIndex(w, r)
		}
	}
}

func createJob() (string, error) {
	cmd := exec.Command("/usr/bin/sh", "-c", "while true; do echo $(date) \"test\"")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	err = cmd.Start()

	jobId := randString(64)
	jobs[jobId] = Job{
		Command:    cmd,
		StdoutPipe: stdout,
		StderrPipe: stderr,
		Socket:     make([]*websocket.Conn, 0),
	}
	return jobId, err
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	if !checkSession(w, r) {
		return
	}

	// get job id and attach

	ws, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(err)

		redirectError(w, r, ERROR_UNKNOWN)
		return
	}

	ws.WriteMessage(websocket.TextMessage, ([]byte)("test"))
	ws.Close()
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
	http.HandleFunc("/ws", wsHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
