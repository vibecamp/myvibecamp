package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/kurrik/oauth1a"
	"github.com/mehanizm/airtable"
)

//go:embed static/*
var static embed.FS

var tmpl *template.Template

type Settings struct {
	ApiKey    string
	ApiSecret string
	Port      int
}

type Session struct {
	key         string
	secret      string
	twitterName string
	twitterID   string
	oauth       *oauth1a.UserConfig
}

var (
	settings *Settings
	service  *oauth1a.Service
	sessions map[string]*Session
)

func NewSessionID() string {
	c := 128
	b := make([]byte, c)
	n, err := io.ReadFull(rand.Reader, b)
	if n != len(b) || err != nil {
		panic("Could not generate random number")
	}
	return base64.URLEncoding.EncodeToString(b)
}

func GetSessionID(r *http.Request) (id string, err error) {
	var c *http.Cookie
	if c, err = r.Cookie("session_id"); err == nil {
		id = c.Value
	}
	return
}

func SessionStartCookie(id string) *http.Cookie {
	return &http.Cookie{
		Name:   "session_id",
		Value:  id,
		MaxAge: 60,
		Secure: false,
		Path:   "/",
	}
}

func SessionEndCookie() *http.Cookie {
	return &http.Cookie{
		Name:   "session_id",
		Value:  "",
		MaxAge: 0,
		Secure: false,
		Path:   "/",
	}
}

func SignInHandler(w http.ResponseWriter, r *http.Request) {
	var (
		url       string
		err       error
		sessionID string
	)
	httpClient := new(http.Client)
	userConfig := &oauth1a.UserConfig{}
	if err = userConfig.GetRequestToken(context.Background(), service, httpClient); err != nil {
		log.Printf("Could not get request token: %v", err)
		http.Error(w, "Problem getting the request token", 500)
		return
	}
	if url, err = userConfig.GetAuthorizeURL(service); err != nil {
		log.Printf("Could not get authorization URL: %v", err)
		http.Error(w, "Problem getting the authorization URL", 500)
		return
	}
	log.Printf("Redirecting user to %v\n", url)
	sessionID = NewSessionID()
	log.Printf("Starting session %v\n", sessionID)
	sessions[sessionID] = &Session{oauth: userConfig}
	http.SetCookie(w, SessionStartCookie(sessionID))
	http.Redirect(w, r, url, 302)
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		token     string
		verifier  string
		sessionID string
		session   *Session
		ok        bool
	)
	log.Printf("Callback hit. %v current sessions.\n", len(sessions))
	if sessionID, err = GetSessionID(r); err != nil {
		log.Printf("Got a callback with no session id: %v\n", err)
		http.Error(w, "No session found", 400)
		return
	}
	if session, ok = sessions[sessionID]; !ok {
		log.Printf("Could not find user config in sesions storage.")
		http.Error(w, "Invalid session", 400)
		return
	}
	if token, verifier, err = session.oauth.ParseAuthorize(r, service); err != nil {
		log.Printf("Could not parse authorization: %v", err)
		http.Error(w, "Problem parsing authorization", 500)
		return
	}
	httpClient := new(http.Client)
	if err = session.oauth.GetAccessToken(context.Background(), token, verifier, service, httpClient); err != nil {
		log.Printf("Error getting access token: %v", err)
		http.Error(w, "Problem getting an access token", 500)
		return
	}

	//log.Printf("Ending session %v.\n", sessionID)
	//delete(sessions, sessionID)
	//http.SetCookie(rw, SessionEndCookie())

	session.key = session.oauth.AccessTokenKey
	session.secret = session.oauth.AccessTokenSecret
	session.twitterName = session.oauth.AccessValues.Get("screen_name")
	session.twitterID = session.oauth.AccessValues.Get("user_id")
	http.Redirect(w, r, "/info", 302)
}

func InfoHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		sessionID string
		session   *Session
		ok        bool
	)

	if sessionID, err = GetSessionID(r); err != nil {
		http.Redirect(w, r, "/?error=No+session+found", 302)
		return
	}
	if session, ok = sessions[sessionID]; !ok {
		http.Redirect(w, r, "/?error=Invalid+session", 302)
		return
	}
	if session.twitterName == "" {
		delete(sessions, sessionID)
		http.SetCookie(w, SessionEndCookie())
		http.Redirect(w, r, "/?error=Invalid+session", 302)
		return
	}

	w.Header().Set("Content-Type", "text/html;charset=utf-8")

	airtableApiKey := os.Getenv("AIRTABLE_API_KEY")
	if airtableApiKey == "" {
		fmt.Println("no api key")
		os.Exit(1)
	}

	client := airtable.NewClient(airtableApiKey)
	table := client.GetTable(os.Getenv("AIRTABLE_DB"), "Attendees")
	records, err := table.GetRecords().
		//FromView("view_1").
		//WithFilterFormula("AND({Field1}='value_1',NOT({Field2}='value_2'))").
		WithFilterFormula(fmt.Sprintf("OR({Twitter Name}='%s',{Twitter Name}='@%s')", session.twitterName, session.twitterName)).
		//WithSort(sortQuery1, sortQuery2).
		ReturnFields("Ticket ID", "Twitter Name", "Cabin").
		InStringFormat("US/Eastern", "en").
		Do()
	if err != nil {
		panic(err)
	}

	if records == nil {
		fmt.Fprintf(w, "ERROR: records is nil")
		return
	} else if len(records.Records) != 1 {
		fmt.Fprintf(w, "ERROR: expected one record, found %d", len(records.Records))
		return
	}

	rec := records.Records[0]
	var cabinMates []string

	if rec.Fields["Cabin"] != "" {
		cRecs, err := table.GetRecords().
			WithFilterFormula(fmt.Sprintf("{Cabin}='%s'", rec.Fields["Cabin"])).
			ReturnFields("Twitter Name").
			InStringFormat("US/Eastern", "en").
			Do()
		if err != nil {
			panic(err)
		}
		for _, c := range cRecs.Records {
			cabinMates = append(cabinMates, fmt.Sprintf("%s", c.Fields["Twitter Name"]))
		}
	}

	tmpl.ExecuteTemplate(w, "info.html.tmpl", struct {
		Name       string
		Cabin      string
		Cabinmates []string
	}{
		Name:       session.twitterName,
		Cabin:      fmt.Sprintf("%s", rec.Fields["Cabin"]),
		Cabinmates: cabinMates,
	})
}

func main() {
	err := godotenv.Load("env")
	if err != nil {
		log.Fatalf("Some error occured. Err: %s", err)
	}

	tmpl = template.Must(template.ParseFS(mustSub(static, "static"), "*.tmpl"))

	sessions = map[string]*Session{}
	settings = &Settings{
		Port:      8080,
		ApiKey:    os.Getenv("TWITTER_API_KEY"),
		ApiSecret: os.Getenv("TWITTER_API_SECRET"),
	}

	if settings.ApiKey == "" || settings.ApiSecret == "" {
		fmt.Fprintf(os.Stderr, "You must specify a consumer key and secret.\n")
		os.Exit(1)
	}

	service = &oauth1a.Service{
		RequestURL:   "https://api.twitter.com/oauth/request_token",
		AuthorizeURL: "https://api.twitter.com/oauth/authorize",
		AccessURL:    "https://api.twitter.com/oauth/access_token",
		ClientConfig: &oauth1a.ClientConfig{
			ConsumerKey:    settings.ApiKey,
			ConsumerSecret: settings.ApiSecret,
			CallbackURL:    fmt.Sprintf("http://127.0.0.1.nip.io:%d/callback", settings.Port),
		},
		Signer: new(oauth1a.HmacSha1Signer),
	}

	http.HandleFunc("/signin/", SignInHandler)
	http.HandleFunc("/callback/", CallbackHandler)
	http.HandleFunc("/info", InfoHandler)
	http.Handle("/", http.FileServer(http.FS(mustSub(static, "static"))))

	log.Printf("Visit http://127.0.0.1.nip.io:%d in your browser\n", settings.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", settings.Port), nil))
}

func mustSub(f embed.FS, path string) fs.FS {
	fsys, err := fs.Sub(f, path)
	if err != nil {
		panic(err)
	}
	return fsys
}
