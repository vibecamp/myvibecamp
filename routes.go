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
	"strconv"

	"github.com/lyoshenka/vibedata/db"
	"github.com/lyoshenka/vibedata/stripe"
	"github.com/lyoshenka/vibedata/fields"

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

	user, err := db.GetUser(session.UserName)
	if err != nil {
		_, err = db.GetSoftLaunchUser(session.UserName)

		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	
		c.Redirect(http.StatusFound, "/ticket-cart")
		return
	}

	c.HTML(http.StatusOK, "index.html.tmpl", user)
}

func CalendarHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.HTML(http.StatusOK, "calendar.html.tmpl", nil)
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.HTML(http.StatusOK, "calendar.html.tmpl", user)
}

func TeamHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.HTML(http.StatusOK, "team.html.tmpl", nil)
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.HTML(http.StatusOK, "team.html.tmpl", user)
}

func ContactUsHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.HTML(http.StatusOK, "contact.html.tmpl", nil)
		return
	}

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.HTML(http.StatusOK, "contact.html.tmpl", user)
}

func TicketCartHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetSoftLaunchUser(session.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "ticketCart.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
			"User": user,
		})
		return
	}

	ticketType := c.PostForm("ticket-type")
	adultTix,_ := strconv.Atoi(c.PostForm("adult-tickets"))
	childTix,_ := strconv.Atoi(c.PostForm("child-tickets"))
	toddlerTix,_ := strconv.Atoi(c.PostForm("toddler-tickets"))
	donationAmount,_ := strconv.Atoi(c.PostForm("donation-amount"))

	if adultTix > user.TicketLimit {
		ErrorFlash(c, fmt.Sprintf("You're limited to %d ticket in the soft launch", user.TicketLimit))
		return
	}

	totalTix := adultTix + childTix + toddlerTix

	// check if they hit any caps here
	// e.g. cabin cap

	var admissionLevel string
	if ticketType == "cabin" {
		admissionLevel = "Cabin"
		cabinCap, err := db.GetConstant(fields.SoftCabinCap)

		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		cabinSold, err := db.GetAggregation(fields.CabinSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if totalTix + cabinSold.Quantity > cabinCap.Value {
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many cabin tickets exceeds our cap! %d cabin tickets left.", cabinCap.Value - cabinSold.Quantity))
			return
		}
		
	} else if ticketType == "tent" {
		admissionLevel = "Tent"
	} else if ticketType == "sat" {
		admissionLevel = "Saturday Night"
	}

	if (ticketType != "sat") {
		salesCap, err := db.GetConstant(fields.SalesCap)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		tixSold, err := db.GetAggregation(fields.TotalTicketsSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		satSold, err := db.GetAggregation(fields.SatSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if totalTix + tixSold.Quantity - satSold.Quantity > salesCap.Value {
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many tickets exceeds our cap! %d tickets left.", salesCap.Value - tixSold.Quantity + satSold.Quantity))
			return
		}
	} else {
		satCap, err := db.GetConstant(fields.SatCap)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		satSold, err := db.GetAggregation(fields.SatSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if totalTix + satSold.Quantity > satCap.Value {
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many tickets exceeds our Saturday night cap! %d Saturday tickets left.", satCap.Value - satSold.Quantity))
			return
		}
	}


	newUser := &db.User{
		AirtableID:			"",
		UserName:			user.UserName,
		TwitterName:		user.TwitterName,
		Name:				user.Name,
		Email:				user.Email,
		AdmissionLevel:		admissionLevel,
		CheckedIn:			false,
		Barcode:			"",
		OrderNotes:			"",
		OrderID:			"",
		TicketID:			"",
		Badge:				c.PostForm("badge-checkbox") == "on",
		Vegetarian:			c.PostForm("vegetarian") == "on",
		GlutenFree:			c.PostForm("glutenfree") == "on",
		LactoseIntolerant:	c.PostForm("lactose") == "on",
		FoodComments:		c.PostForm("comments"),
	}

	err = newUser.CreateUser()
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	params := url.Values{}
	params.Set("ticketType", ticketType)
	params.Set("adult", strconv.Itoa(adultTix))
	params.Set("child", strconv.Itoa(childTix))
	params.Set("toddler", strconv.Itoa(toddlerTix))
	params.Set("donation", strconv.Itoa(donationAmount))

	c.Redirect(http.StatusFound, "/checkout"+"?"+params.Encode())
}

func SoftLaunchSignIn(c *gin.Context) {
	session := GetSession(c)
	
	if c.Request.Method == http.MethodGet {
		if !session.SignedIn() {
			c.HTML(http.StatusOK, "softLaunchSignIn.html.tmpl", nil)
			return
		}

		user, err := db.GetSoftLaunchUser(session.UserName)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		// redirect to cart?
		c.HTML(http.StatusOK, "softLaunchSignIn.html.tmpl", user)
		return
	}

	// I need to setup session stuff for this - oauth email?
	// or just get user by email & handle the rest like their
	// email is their twitter
	// magic email links? not hard really but need a way to send emails

	emailAddr := c.PostForm("email-address")
	// get user by email somehow
	user, err := db.GetSoftLaunchUser(emailAddr)
	// then return the same page
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	session.UserName = user.UserName
	session.TwitterName = user.UserName
	session.TwitterID = user.AirtableID
	session.Oauth = nil
	SaveSession(c, session)

	c.HTML(http.StatusOK, "softLaunchSignIn.html.tmpl", user)
	return
}

func StripeCheckoutHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetUser(session.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ticketIds := []string{"adult", "child", "toddler", "donation"}
	ticketType := c.Query("ticketType")
	var items []stripe.Item
	for ind, element := range ticketIds {
		amt,_ := strconv.Atoi(c.Query(element))
		if amt > 0 {
			if ind < 3 {
				items = append(items, stripe.Item{
					Id: element+"-"+ticketType,
					Quantity: amt, 
					Amount: 0,
				})
			} else {
				items = append(items, stripe.Item{
					Id: ticketIds[ind], 
					Quantity: 1, 
					Amount: amt,
				})
			}
		}
	}

	// log.Debugf("%v", items)
	itemMap := struct {
		Items []stripe.Item `json:"items"`
	}{
		Items: items,
	}
	// log.Debugf("%v", itemMap)
	itemJson,err := json.Marshal(itemMap)	
	if err != nil {
		log.Errorf("%v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	// log.Debugf(string(itemJson))

	c.HTML(http.StatusOK, "checkout.html.tmpl", gin.H{
		"User": user,
		"Items": string(itemJson),
	})
}

func PurchaseCompleteHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetUser(session.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	log.Debugf("%v", user)

	order, err := db.GetOrder(user.OrderID)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.HTML(http.StatusOK, "purchaseComplete.html.tmpl", gin.H{
		"User": user,
		"Order": order,
	})
}

func Logistics2023Handler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetUser(session.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	order, err := db.GetOrder(user.OrderID)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "logistics2023.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
			"User": user,
			"Order": order,
		})
		return
	}

	badge := c.PostForm("badge-checkbox") == "on"
	vegetarian := c.PostForm("vegetarian") == "on"
	glutenFree := c.PostForm("glutenfree") == "on"
	lactoseIntolerant := c.PostForm("lactose") == "on"
	foodComments :=	c.PostForm("comments")

	err = user.Set2023Logistics(badge, vegetarian, glutenFree, lactoseIntolerant, foodComments)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	SuccessFlash(c, "Saved!")

	c.Redirect(http.StatusFound, "/2023-logistics")
	return
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

	// cabinMates, err := user.GetCabinMates()
	var cabinMates []string
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
	if (badgeChoice == "yes") != user.Badge {
		if user.Badge && badgeChoice == "no" {
			switchedFromYesToNo = true
		}
		err = user.SetBadge(badgeChoice)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	params := url.Values{}
	// params.Set("cabin", user.Cabin)
	params.Set("handle", user.TwitterName)

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret != "" {
		h := hmac.New(sha256.New, []byte(hmacSecret))
		h.Write([]byte(fmt.Sprintf("%s", user.TwitterName)))
		params.Set("hmac", strings.TrimRight(base64.URLEncoding.EncodeToString(h.Sum(nil)), "="))
	}

	badgeDomain := "https://that-part-of-twitter.herokuapp.com"

	if !user.Badge {
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
