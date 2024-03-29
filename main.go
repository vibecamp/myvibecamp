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

	"github.com/vibecamp/myvibecamp/db"
	"github.com/vibecamp/myvibecamp/stripe"

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
	externalURL  string
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

	externalURL = os.Getenv("EXTERNAL_URL")
	var (
		port                 = os.Getenv("PORT")
		apiKey               = os.Getenv("TWITTER_API_KEY")
		apiSecret            = os.Getenv("TWITTER_API_SECRET")
		stripeApiKey         = os.Getenv("STRIPE_API_KEY")
		stripePublishableKey = os.Getenv("STRIPE_PUBLISHABLE_KEY")
		stripeWebhookSecret  = os.Getenv("STRIPE_WEBHOOK_SECRET")
		klaviyoKey           = os.Getenv("KLAVIYO_API_KEY")
		klaviyoListId        = os.Getenv("KLAVIYO_LIST_ID")
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

	if stripeApiKey == "" || stripePublishableKey == "" {
		log.Errorf("No stripe API key\n")
		os.Exit(1)
	}

	if os.Getenv("AIRTABLE_API_KEY") == "" || os.Getenv("AIRTABLE_BASE_ID") == "" ||
		os.Getenv("AIRTABLE_TABLE_NAME") == "" || os.Getenv("AIRTABLE_2023_BASE") == "" ||
		os.Getenv("AIRTABLE_SL_TABLE") == "" || os.Getenv("AIRTABLE_ATTENDEE_TABLE") == "" ||
		os.Getenv("AIRTABLE_CONSTANTS_TABLE") == "" || os.Getenv("AIRTABLE_AGG_TABLE") == "" ||
		os.Getenv("AIRTABLE_ORDER_TABLE") == "" {
		log.Errorf("need all AIRTABLE_ env vars set")
		os.Exit(1)
	}

	cacheTime := 24 * time.Hour
	if localDevMode {
		cacheTime = 1 * time.Second
	}
	c := cache.New(cacheTime, 1*time.Hour)

	db.Init(os.Getenv("AIRTABLE_API_KEY"), os.Getenv("AIRTABLE_BASE_ID"), c)

	if localDevMode {
		stripe.Init("sk_test_4eC39HqLyjWDarjtT1zdp7dc", "", klaviyoKey, klaviyoListId)
	} else {
		stripe.Init(stripeApiKey, stripeWebhookSecret, klaviyoKey, klaviyoListId)
	}

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
	r.GET("/signin-redirect", SignInRedirect)
	r.GET("/calendar", CalendarHandler)

	r.GET("/ticket", TicketHandler)
	r.GET("/logistics", LogisticsHandler)
	r.GET("/badge", BadgeHandler)
	r.POST("/badge", BadgeHandler)
	r.GET("/food", FoodHandler)
	r.POST("/food", FoodHandler)
	r.GET("/cabinlist", CabinListHandler)
	r.GET("/checkin/:ticketId", CheckinHandler)
	r.POST("/checkin/:ticketId", CheckinHandler)
	r.GET("/checkout", StripeCheckoutHandler)
	r.POST("/create-payment-intent", stripe.HandleCreatePaymentIntent)
	r.POST("/create-payment-intent-transport", stripe.HandleTransportCreatePaymentIntent)
	r.GET("/ticket-cart", TicketCartHandler)
	r.POST("/ticket-cart", TicketCartHandler)
	r.GET("/vc2-sl", SoftLaunchSignIn)
	r.POST("/vc2-sl", SoftLaunchSignIn)
	r.POST("/stripe-webhook", stripe.HandleStripeWebhook)
	r.GET("/checkout-complete", PurchaseCompleteHandler)
	r.GET("/checkout-failed", PurchaseFailedHandler)
	r.POST("/checkout-failed", PurchaseFailedHandler)
	r.GET("/2023-logistics", Logistics2023Handler)
	r.POST("/2023-logistics", Logistics2023Handler)
	r.GET("/transport-checkout", TransportCheckoutHandler)
	r.GET("/2023-transport", Transport2023Handler)
	r.POST("/2023-transport", Transport2023Handler)
	r.GET("/chaos-mode", ChaosModeSignIn)
	r.POST("/chaos-mode", ChaosModeSignIn)
	r.GET("/chaos-cart", ChaosModeCartHandler)
	r.POST("/chaos-cart", ChaosModeCartHandler)
	r.GET("/auth-discord", DiscordAuthenticator)
	r.GET("/app-user", AppEndpoint)
	r.GET("/user-by-discord", UserByDiscordEndpoint)
	r.GET("/attendees", GetAttendeesEndpoint)
	r.GET("/sponsorship-cart", SponsorshipCartHandler)
	r.POST("/sponsorship-cart", SponsorshipCartHandler)
	r.GET("/vc2", VC2Welcome)
	r.POST("/vc2", VC2Welcome)
	r.GET("/vc2-ticket", VC2TicketHandler)

	r.GET("/", IndexHandler)
	r.StaticFS("/css", http.FS(mustSub(static, "static/css")))
	r.StaticFS("/js", http.FS(mustSub(static, "static/js")))
	r.StaticFS("/img", http.FS(mustSub(static, "static/img")))

	log.Printf("Visit %s in your browser\n", externalURL)

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

	if !localDevMode {
		go func() {
			time.Sleep(5 * time.Second)
			db.CacheWarmup()
		}()
	}

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
