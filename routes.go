package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
  	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"bytes"
	"io"

	"github.com/lyoshenka/vibedata/db"

  	"github.com/stripe/stripe-go/v72"
  	"github.com/stripe/stripe-go/v72/paymentintent"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
)

func IndexHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.HTML(http.StatusOK, "index.html.tmpl", nil)
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.HTML(http.StatusOK, "index.html.tmpl", user)
}

type item struct {
	id string
  }
  
func calculateOrderAmount(items []item) int64 {
	// Replace this constant with a calculation of the order's amount
	// Calculate the order total on the server to prevent
	// people from directly manipulating the amount on the client
	return 1400
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  log.Printf("json.NewEncoder.Encode: %v", err)
	  return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, &buf); err != nil {
	  log.Printf("io.Copy: %v", err)
	  return
	}
  }

func StripeCheckoutHandler(c *gin.Context) {
	session := GetSession(c)

	if !session.SignedIn() {
		c.HTML(http.StatusOK, "checkout.html.tmpl", nil)
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.HTML(http.StatusOK, "checkout.html.tmpl", user)
}

func CreatePaymentIntentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
	  http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	  return
	}
  
	var req struct {
	  Items []item `json:"items"`
	}
  
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  log.Printf("json.NewDecoder.Decode: %v", err)
	  return
	}
  
	// Create a PaymentIntent with amount and currency
	params := &stripe.PaymentIntentParams{
	  Amount:   stripe.Int64(calculateOrderAmount(req.Items)),
	  Currency: stripe.String(string(stripe.CurrencyEUR)),
	  AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
		Enabled: stripe.Bool(true),
	  },
	}
  
	pi, err := paymentintent.New(params)
	log.Printf("pi.New: %v", pi.ClientSecret)
  
	if err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  log.Printf("pi.New: %v", err)
	  return
	}
  
	writeJSON(w, struct {
	  ClientSecret string `json:"clientSecret"`
	}{
	  ClientSecret: pi.ClientSecret,
	})
}

func TicketHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ticketGroup, err := user.GetTicketGroup()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	qr, err := qrcode.Encode(fmt.Sprintf(`%s/checkin/%s`, externalURL, user.Barcode), qrcode.Medium, 256)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "generating qr code"))
		return
	}

	c.HTML(http.StatusOK, "ticket.html.tmpl", struct {
		Name        string
		QR          string
		TicketGroup []*db.User
	}{
		Name:        session.TwitterName,
		QR:          base64.StdEncoding.EncodeToString(qr),
		TicketGroup: ticketGroup,
	})
}

func LogisticsHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	cabinMates, err := user.GetCabinMates()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "logistics.html.tmpl", struct {
		User       *db.User
		CabinMates []string
	}{
		User:       user,
		CabinMates: cabinMates,
	})
}

func CheckinHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	} else if !localDevMode && !user.HasCheckinPermission() {
		c.AbortWithError(http.StatusForbidden, errors.New("This page is for staff only"))
		return
	}

	barcode := c.Param("barcode")
	barcodeUser, err := db.GetUserFromBarcode(barcode)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ticketGroup, err := barcodeUser.GetTicketGroup()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	anyUnchecked := false
	for _, u := range ticketGroup {
		if !u.CheckedIn {
			anyUnchecked = true
			break
		}
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "checkin.html.tmpl", gin.H{
			"flashes":      GetFlashes(c),
			"group":        ticketGroup,
			"anyUnchecked": anyUnchecked,
		})
		return
	}

	checkinCount := 0
	for _, n := range ticketGroup {
		if checkedIn := c.PostForm(n.TwitterName); checkedIn == "on" {
			u, err := db.GetUser(n.TwitterName)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			u.SetCheckedIn()
			checkinCount++
		}
	}

	if checkinCount == 0 {
		WarningFlash(c, "Select at least one person to check in")
	} else if checkinCount == 1 {
		SuccessFlash(c, "Checked in 1 person")
	} else {
		SuccessFlash(c, fmt.Sprintf("Checked in %d people", checkinCount))
	}

	c.Redirect(http.StatusFound, "/checkin/"+barcode)
}

func BadgeHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "badge.html.tmpl", struct {
			User *db.User
		}{
			User: user,
		})
		return
	}

	switchedFromYesToNo := false

	badgeChoice := c.PostForm("badge")
	if badgeChoice != user.Badge {
		if user.Badge == "yes" && badgeChoice == "no" {
			switchedFromYesToNo = true
		}
		err = user.SetBadge(badgeChoice)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	params := url.Values{}
	params.Set("cabin", user.Cabin)
	params.Set("handle", user.TwitterName)

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret != "" {
		h := hmac.New(sha256.New, []byte(hmacSecret))
		h.Write([]byte(fmt.Sprintf("%s|%s", user.Cabin, user.TwitterName)))
		params.Set("hmac", strings.TrimRight(base64.URLEncoding.EncodeToString(h.Sum(nil)), "="))
	}

	badgeDomain := "https://that-part-of-twitter.herokuapp.com"

	if user.Badge == "no" {
		if switchedFromYesToNo && !localDevMode {
			go func() {
				_, err := http.Get(badgeDomain + "/delete?" + params.Encode())
				if err != nil {
					log.Errorf("hitting delete api: %+v", err)
				}
			}()
		}

		c.Redirect(http.StatusFound, "/badge")
		return
	}

	c.Redirect(http.StatusFound, badgeDomain+"?"+params.Encode())
}

func FoodHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "food.html.tmpl", gin.H{
			"User":    user,
			"flashes": GetFlashes(c),
		})
		return
	}

	err = user.SetFood(
		c.PostForm("vegetarian") == "on",
		c.PostForm("glutenfree") == "on",
		c.PostForm("lactose") == "on",
		c.PostForm("comments"),
	)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	SuccessFlash(c, "Saved!")

	c.Redirect(http.StatusFound, "/food")
	return
}

func CabinListHandler(c *gin.Context) {
	authToken := c.Query("auth_token")

	if authToken == "" {
		c.AbortWithError(http.StatusUnauthorized, errors.New("auth_token required"))
		return
	}

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		c.AbortWithError(http.StatusForbidden, errors.New("route disabled"))
		return
	}

	h := sha256.Sum256([]byte(hmacSecret))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(authToken)) != 1 {
		c.AbortWithError(http.StatusForbidden, errors.New("invalid auth_token"))
		return
	}

	cabins, err := db.GetCabinsForBadgeGenerator()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, cabins)
}
