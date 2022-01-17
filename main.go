package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/cockroachdb/errors/oserror"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/kurrik/oauth1a"
	log "github.com/sirupsen/logrus"
)

//go:embed static/*
var static embed.FS

var (
	localDevMode bool
	service      *oauth1a.Service
)

func main() {
	http.DefaultClient.Timeout = 10 * time.Second

	// load env file if exists
	_, err := os.Open("env")
	if !oserror.IsNotExist(err) {
		err := godotenv.Load("env")
		if err != nil {
			log.Fatalf("loading env: %s", err)
		}
	}

	var (
		externalURL = os.Getenv("EXTERNAL_URL")
		port        = os.Getenv("PORT")
		apiKey      = os.Getenv("TWITTER_API_KEY")
		apiSecret   = os.Getenv("TWITTER_API_SECRET")
	)

	localDevMode = os.Getenv("DEV") == "true"
	if localDevMode {
		log.SetLevel(log.DebugLevel) // we have TraceLevel messages as well
		log.Println("dev mode enabled")
	}

	if apiKey == "" || apiSecret == "" {
		log.Errorf("You must specify a consumer key and secret.\n")
		os.Exit(1)
	}

	service = &oauth1a.Service{
		RequestURL:   "https://api.twitter.com/oauth/request_token",
		AuthorizeURL: "https://api.twitter.com/oauth/authorize",
		AccessURL:    "https://api.twitter.com/oauth/access_token",
		ClientConfig: &oauth1a.ClientConfig{
			ConsumerKey:    apiKey,
			ConsumerSecret: apiSecret,
			CallbackURL:    fmt.Sprintf("%s/callback", externalURL),
		},
		Signer: new(oauth1a.HmacSha1Signer),
	}

	r := gin.Default()

	store := memstore.NewStore([]byte("whatever")) // TODO: add keys?
	store.Options(sessions.Options{Path: "/", MaxAge: 3600, Secure: !localDevMode, HttpOnly: true})
	r.Use(sessions.Sessions("session_id", store))

	tmpl := template.Must(template.ParseFS(mustSub(static, "static"), "*.tmpl"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/signin", SignInHandler)
	r.GET("/callback", CallbackHandler)
	r.GET("/info", InfoHandler)
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html.tmpl", nil)
	})
	r.StaticFS("/css", http.FS(mustSub(static, "static/css")))

	log.Printf("Visit %s in your browser\n", externalURL)
	//log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
	log.Fatal(r.Run(fmt.Sprintf(":%s", port)))
}

func mustSub(f embed.FS, path string) fs.FS {
	fsys, err := fs.Sub(f, path)
	if err != nil {
		panic(err)
	}
	return fsys
}
