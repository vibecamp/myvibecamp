package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lyoshenka/vibedata/db"

	"github.com/cockroachdb/errors/oserror"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/kurrik/oauth1a"
	"github.com/patrickmn/go-cache"
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
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	if apiKey == "" || apiSecret == "" {
		log.Errorf("You must specify a consumer key and secret.\n")
		os.Exit(1)
	}

	if os.Getenv("AIRTABLE_API_KEY") == "" || os.Getenv("AIRTABLE_BASE_ID") == "" ||
		os.Getenv("AIRTABLE_TABLE_NAME") == "" {
		log.Errorf("need all three AIRTABLE_ env vars set")
		os.Exit(1)
	}

	cacheTime := 15 * time.Minute
	if localDevMode {
		cacheTime = 1 * time.Second
	}
	c := cache.New(cacheTime, 1*time.Hour)

	db.Init(os.Getenv("AIRTABLE_API_KEY"), os.Getenv("AIRTABLE_BASE_ID"), os.Getenv("AIRTABLE_TABLE_NAME"), c)

	callbackUrl := fmt.Sprintf("%s/callback", externalURL)
	log.Println("Twitter callback URL: ", callbackUrl)
	service = &oauth1a.Service{
		RequestURL:   "https://api.twitter.com/oauth/request_token",
		AuthorizeURL: "https://api.twitter.com/oauth/authorize",
		AccessURL:    "https://api.twitter.com/oauth/access_token",
		ClientConfig: &oauth1a.ClientConfig{
			ConsumerKey:    apiKey,
			ConsumerSecret: apiSecret,
			CallbackURL:    callbackUrl,
		},
		Signer: new(oauth1a.HmacSha1Signer),
	}

	r := gin.Default()
	r.Use(errPrinter)

	cookieAuthKey := sha256.Sum256([]byte(os.Getenv("COOKIE_SECRET") + "authentication key"))
	cookieEncKey := sha256.Sum256([]byte(os.Getenv("COOKIE_SECRET") + "encryption key"))
	store := cookie.NewStore(cookieAuthKey[:], cookieEncKey[:])
	store.Options(sessions.Options{Path: "/", MaxAge: 60 * 60 * 24 * 7, Secure: !localDevMode, HttpOnly: true})
	r.Use(sessions.Sessions("session_id", store))

	tmpl := template.Must(template.ParseFS(mustSub(static, "static"), "*.tmpl"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/signin", SignInHandler)
	r.GET("/signout", SignOutHandler)
	r.GET("/callback", CallbackHandler)

	r.GET("/ticket", TicketHandler)
	r.GET("/logistics", LogisticsHandler)
	r.POST("/badge", BadgeHandler)
	r.GET("/checkin/:barcode", CheckinHandler)

	r.GET("/", IndexHandler)
	r.StaticFS("/css", http.FS(mustSub(static, "static/css")))
	r.StaticFS("/js", http.FS(mustSub(static, "static/js")))
	r.StaticFS("/img", http.FS(mustSub(static, "static/img")))

	log.Printf("Visit %s in your browser\n", externalURL)
	//log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
	//log.Fatal(r.Run(fmt.Sprintf(":%s", port)))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}
	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			log.Errorf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 5 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %+v", err)
	}
	log.Println("Server exiting")
}

func mustSub(f embed.FS, path string) fs.FS {
	fsys, err := fs.Sub(f, path)
	if err != nil {
		panic(err)
	}
	return fsys
}

func errPrinter(c *gin.Context) {
	c.Next()

	if len(c.Errors) > 0 {
		errorStrings := make([]string, len(c.Errors))
		for i, err := range c.Errors {
			if localDevMode {
				errorStrings[i] = fmt.Sprintf("%+v", err.Unwrap())
			} else {
				errorStrings[i] = fmt.Sprintf("%v", err.Unwrap())
			}
		}
		c.HTML(400, "errorList.html.tmpl", errorStrings)
	}
}
