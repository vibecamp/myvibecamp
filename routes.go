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
	"strconv"
	"strings"

	"github.com/vibecamp/myvibecamp/db"
	"github.com/vibecamp/myvibecamp/fields"

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

	findUser(c, session.UserName, false)
}

func VC2Welcome(c *gin.Context) {
	session := GetSession(c)
	if c.Request.Method == http.MethodGet {
		if !session.SignedIn() {
			c.HTML(http.StatusOK, "vc2welcome.html.tmpl", nil)
			return
		}

		findUser(c, session.UserName, false)
		return
	}

	// I need to setup session stuff for this - oauth email?
	// or just get user by email & handle the rest like their
	// email is their twitter
	// magic email links? not hard really but need a way to send emails

	emailAddr := c.PostForm("email-address")
	if !localDevMode && !strings.Contains(emailAddr, "@") {
		// throw an error if it's not a valid email on prod
		c.AbortWithError(http.StatusBadRequest, errors.New("must enter a valid email address"))
		return
	}

	findUser(c, emailAddr, true)
}

func findUser(c *gin.Context, username string, isEmailSignIn bool) {
	user, err := db.GetUser(username)
	if err == nil && user != nil {
		if isEmailSignIn {
			makeEmailSession(c, user.UserName, user.AirtableID)
		}

		// if they have an order ID, check the order
		if len(user.OrderID) > 0 {
			order, err := db.GetOrder(user.OrderID)
			// if it exists and is not blank payment status, direct them by payment status
			if err == nil && order != nil && order.PaymentStatus != "" {
				switch order.PaymentStatus {
				case "failed":
					c.Redirect(http.StatusFound, "/checkout-failed")
					return
				case "success":
					c.Redirect(http.StatusFound, "/2023-transport")
					return
				case "processing":
					c.Redirect(http.StatusFound, "/checkout-complete")
					return
				}
			} else {
				// otherwise send them based on their ticket path to the cart
				if user.TicketPath == "Sponsorship" {
					c.Redirect(http.StatusFound, "/sponsorship-cart")
					return
				} else if user.TicketPath == "FCFS" || user.TicketPath == "Lottery" || user.TicketPath == "Application" || user.TicketPath == fields.LateApp {
					c.Redirect(http.StatusFound, "/chaos-cart")
					return
				} else {
					c.Redirect(http.StatusFound, "/ticket-cart")
					return
				}
			}
		} else if user.TicketPath == "Sponsorship" {
			// if they dont have an order ID, check if they're in sponsorship table. if not, they're a full sponsor
			sponsoredUser, err := db.GetSponsorshipUser(username)
			if sponsoredUser == nil && err != nil {
				c.Redirect(http.StatusFound, "/2023-transport")
				return
			}
		} else if user.AdmissionLevel == fields.Staff || user.TicketPath == fields.Volunteer || user.TicketPath == fields.TicketSwap || user.TicketPath == fields.Comped {
			// if they don't have an order id check if they're staff
			c.Redirect(http.StatusFound, "/2023-transport")
			return
		}
	}

	// check for sponsorship first, in case they're both on sponsorship and e.g. soft launch
	sponsoredUser, err := db.GetSponsorshipUser(username)
	if err == nil && sponsoredUser != nil {
		if isEmailSignIn {
			makeEmailSession(c, sponsoredUser.UserName, sponsoredUser.AirtableID)
		}
		c.Redirect(http.StatusFound, "/sponsorship-cart")
		return
	}

	// check if they're a soft launch
	softLaunchUser, err := db.GetSoftLaunchUser(username)
	if err == nil && softLaunchUser != nil {
		if isEmailSignIn {
			makeEmailSession(c, softLaunchUser.UserName, softLaunchUser.AirtableID)
		}
		c.Redirect(http.StatusFound, "/vc2-sl")
		return
	}

	// check if they're chaos user
	chaosUser, err := db.GetChaosUser(username)
	if err == nil && chaosUser != nil {
		if isEmailSignIn {
			makeEmailSession(c, chaosUser.UserName, chaosUser.AirtableID)
		}
		c.Redirect(http.StatusFound, "/chaos-mode")
		return
	}

	// abort
	c.AbortWithError(http.StatusBadRequest, err)
}

func makeEmailSession(c *gin.Context, username string, id string) {
	session := GetSession(c)
	session.UserName = username
	session.TwitterName = username
	session.TwitterID = id
	session.Oauth = nil
	SaveSession(c, session)
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

func VC2TicketHandler(c *gin.Context) {
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

	c.HTML(http.StatusOK, "vibecamp2.html.tmpl", user)
}

func Transport2023Handler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	_, err := db.GetUser(session.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if c.Request.Method == "POST" {
		busQuantity, _ := strconv.Atoi(c.PostForm("busQuantity"))
		busToVibecamp := c.PostForm("busArriveTime")
		busFromVibecamp := c.PostForm("busDepartTime")
		sleepingBags := 0
		pillows := 0
		sheetSets := 0

		if busQuantity > 0 {
			if busToVibecamp == "" && busFromVibecamp == "" {
				c.AbortWithError(http.StatusBadRequest, errors.New("must select a bus slot"))
				return
			}

			if busToVibecamp != "" {
				bs, err := db.GetSlot(busToVibecamp)
				if err != nil {
					c.AbortWithError(http.StatusBadRequest, err)
					return
				}

				if bs.Purchased+busQuantity > bs.Cap {
					c.AbortWithError(http.StatusBadRequest, errors.New("bus slot is full"))
					return
				}
			}

			if busFromVibecamp != "" {
				bs, err := db.GetSlot(busFromVibecamp)
				if err != nil {
					c.AbortWithError(http.StatusBadRequest, err)
					return
				}

				if bs.Purchased+busQuantity > bs.Cap {
					c.AbortWithError(http.StatusBadRequest, errors.New("bus slot is full"))
					return
				}
			}
		} else {
			busToVibecamp = ""
			busFromVibecamp = ""
		}

		params := url.Values{}
		params.Set("busQuantity", strconv.Itoa(busQuantity))
		params.Set("sleepingBags", strconv.Itoa(sleepingBags))
		params.Set("pillows", strconv.Itoa(pillows))
		params.Set("sheetSets", strconv.Itoa(sheetSets))
		params.Set("busToVibecamp", busToVibecamp)
		params.Set("busFromVibecamp", busFromVibecamp)

		c.Redirect(http.StatusFound, "/transport-checkout"+"?"+params.Encode())
		return
	}

	c.HTML(http.StatusOK, "transport2023.html.tmpl", gin.H{})
}

func TransportCheckoutHandler(c *gin.Context) {
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

	busSpots, _ := strconv.Atoi(c.Query("busQuantity"))
	busToVibecamp := c.Query("busToVibecamp")
	busFromVibecamp := c.Query("busFromVibecamp")
	sleepingBags, _ := strconv.Atoi(c.Query("sleepingBags"))
	pillows, _ := strconv.Atoi(c.Query("pillows"))
	sheetSets, _ := strconv.Atoi(c.Query("sheetSets"))
	items := struct {
		BusSpots        int    `json:"busSpots"`
		BusToVibecamp   string `json:"busToVibecamp"`
		BusFromVibecamp string `json:"busFromVibecamp"`
		SleepingBags    int    `json:"sleepingBags"`
		SheetSets       int    `json:"sheetSets"`
		Pillows         int    `json:"pillows"`
	}{
		BusSpots:        busSpots,
		BusToVibecamp:   busToVibecamp,
		BusFromVibecamp: busFromVibecamp,
		SleepingBags:    sleepingBags,
		SheetSets:       sheetSets,
		Pillows:         pillows,
	}

	// log.Debugf("%v", itemMap)
	itemJson, err := json.Marshal(items)
	if err != nil {
		log.Errorf("%v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	// log.Debugf(string(itemJson))

	c.HTML(http.StatusOK, "transportCheckout.html.tmpl", gin.H{
		"User":  user,
		"Items": string(itemJson),
	})
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

	if user.TicketLimit < 1 {
		c.HTML(http.StatusOK, "ticketSalesClosed.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
		})
		return
	}

	attendee, err := db.GetUser(session.UserName)
	if err == nil && attendee != nil {
		if attendee.OrderID != "" {
			order, err := db.GetOrder(attendee.OrderID)
			if err == nil && order != nil && order.PaymentStatus == "success" {
				c.Redirect(http.StatusFound, "/2023-logistics")
				return
			}
		}
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "ticketCart.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
			"User":    user,
		})
		return
	}

	ticketType := c.PostForm("ticket-type")
	adultTix, _ := strconv.Atoi(c.PostForm("adult-tickets"))
	childTix, _ := strconv.Atoi(c.PostForm("child-tickets"))
	toddlerTix, _ := strconv.Atoi(c.PostForm("toddler-tickets"))
	donationAmount, _ := strconv.Atoi(c.PostForm("donation-amount"))

	if adultTix > user.TicketLimit {
		ErrorFlash(c, fmt.Sprintf("You're limited to %d ticket in the soft launch", user.TicketLimit))
		return
	}

	totalTix := adultTix + childTix + toddlerTix
	dbTicketType := ""

	if adultTix > 0 {
		dbTicketType = "Adult"
	} else if childTix > 0 {
		dbTicketType = "Child"
	} else if toddlerTix > 0 {
		dbTicketType = "Toddler"
	}

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

		if totalTix+cabinSold.Quantity > cabinCap.Value {
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many cabin tickets exceeds our cap! %d cabin tickets left.", cabinCap.Value-cabinSold.Quantity))
			return
		}

	} else if ticketType == "tent" {
		admissionLevel = "Tent"
	} else if ticketType == "sat" {
		admissionLevel = "Saturday Night"
	}

	if ticketType != "sat" {
		salesCap, err := db.GetConstant(fields.SalesCap)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		fullTixSold, err := db.GetAggregation(fields.FullSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if totalTix+fullTixSold.Quantity > salesCap.Value {
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many tickets exceeds our cap! %d tickets left.", salesCap.Value-fullTixSold.Quantity))
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

		if totalTix+satSold.Quantity > satCap.Value {
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many tickets exceeds our Saturday night cap! %d Saturday tickets left.", satCap.Value-satSold.Quantity))
			return
		}
	}

	newUser := &db.User{
		AirtableID:         "",
		UserName:           user.UserName,
		TwitterName:        user.TwitterName,
		Name:               user.Name,
		Email:              user.Email,
		Cabin2022:          user.Cabin2022,
		AdmissionLevel:     admissionLevel,
		TicketType:         dbTicketType,
		CheckedIn:          false,
		Barcode:            "",
		OrderNotes:         "",
		OrderID:            "",
		TicketID:           "",
		Badge:              c.PostForm("badge-checkbox") == "on",
		Vegetarian:         c.PostForm("vegetarian") == "on",
		GlutenFree:         c.PostForm("glutenfree") == "on",
		LactoseIntolerant:  c.PostForm("lactose") == "on",
		FoodComments:       c.PostForm("comments"),
		DiscordName:        c.PostForm("discord-name"),
		TicketPath:         "2022 Attendee",
		SponsorshipConfirm: false,
	}

	if attendee != nil {
		newUser.OrderID = attendee.OrderID
		newUser.AirtableID = attendee.AirtableID
		err = newUser.UpdateUser()
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	} else {
		err = newUser.CreateUser()
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	params := url.Values{}
	params.Set("ticketType", ticketType)
	params.Set("adult", strconv.Itoa(adultTix))
	params.Set("child", strconv.Itoa(childTix))
	params.Set("toddler", strconv.Itoa(toddlerTix))
	params.Set("donation", strconv.Itoa(donationAmount))

	c.Redirect(http.StatusFound, "/checkout"+"?"+params.Encode())
}

func SponsorshipCartHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetSponsorshipUser(session.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if user.TicketLimit < 1 {
		c.HTML(http.StatusOK, "ticketSalesClosed.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
		})
		return
	}

	attendee, err := db.GetUser(session.UserName)
	if err == nil && attendee != nil {
		if attendee.OrderID != "" {
			order, err := db.GetOrder(attendee.OrderID)
			if err == nil && order != nil && order.PaymentStatus == "success" {
				c.Redirect(http.StatusFound, "/checkout-complete")
				return
			}
			// add pending status page?
		}
	}

	adultTix := 1
	dbTicketType := "Adult"
	admissionLevel := user.AdmissionLevel
	var basePrice float64
	var ticketType string
	if admissionLevel == "Tent" {
		basePrice = float64(420.69)
		ticketType = "tent"

		salesCap, err := db.GetConstant(fields.SalesCap)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		fullTixSold, err := db.GetAggregation(fields.FullSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if adultTix+fullTixSold.Quantity > salesCap.Value {
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many tickets exceeds our cap! %d tickets left.", salesCap.Value-fullTixSold.Quantity))
			return
		}
	} else {
		basePrice = float64(140)
		ticketType = "sat"

		satCap, err := db.GetConstant(fields.SatCap)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		satTixSold, err := db.GetAggregation(fields.SatSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if adultTix+satTixSold.Quantity > satCap.Value {
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many tickets exceeds our cap! %d tickets left.", satCap.Value-satTixSold.Quantity))
			return
		}
	}

	subtotalFloat := basePrice - user.Discount.ToFloat()
	feeFloat := subtotalFloat * float64(0.03)
	subtotal := db.CurrencyFromFloat(subtotalFloat)
	total := db.CurrencyFromFloat(subtotalFloat + feeFloat)
	fee := db.CurrencyFromFloat(feeFloat)

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "sponsorshipCart.html.tmpl", gin.H{
			"flashes":  GetFlashes(c),
			"User":     user,
			"Total":    total,
			"Fee":      fee,
			"Subtotal": subtotal,
		})
		return
	}

	newUser := &db.User{
		AirtableID:         "",
		UserName:           user.UserName,
		TwitterName:        user.TwitterName,
		Name:               user.Name,
		Email:              user.Email,
		AdmissionLevel:     admissionLevel,
		TicketType:         dbTicketType,
		CheckedIn:          false,
		Barcode:            "",
		OrderNotes:         "",
		OrderID:            "",
		TicketID:           "",
		SponsorshipConfirm: false,
		Badge:              c.PostForm("badge-checkbox") == "on",
		Vegetarian:         c.PostForm("vegetarian") == "on",
		GlutenFree:         c.PostForm("glutenfree") == "on",
		LactoseIntolerant:  c.PostForm("lactose") == "on",
		FoodComments:       c.PostForm("comments"),
		DiscordName:        c.PostForm("discord-name"),
		TicketPath:         "Sponsorship",
	}

	if attendee != nil {
		newUser.OrderID = attendee.OrderID
		newUser.AirtableID = attendee.AirtableID
		err = newUser.UpdateUser()
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	} else {
		err = newUser.CreateUser()
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	params := url.Values{}
	params.Set("ticketType", ticketType)
	params.Set("adult", strconv.Itoa(adultTix))
	params.Set("child", "0")
	params.Set("toddler", "0")
	params.Set("donation", "0")

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

		if user.TicketLimit < 1 {
			c.HTML(http.StatusOK, "ticketSalesClosed.html.tmpl", gin.H{
				"flashes": GetFlashes(c),
			})
			return
		}

		c.HTML(http.StatusOK, "softLaunchSignIn.html.tmpl", user)
		return
	}

	// I need to setup session stuff for this - oauth email?
	// or just get user by email & handle the rest like their
	// email is their twitter
	// magic email links? not hard really but need a way to send emails

	emailAddr := c.PostForm("email-address")
	if !localDevMode && !strings.Contains(emailAddr, "@") {
		// throw an error if it's not a valid email on prod
		c.AbortWithError(http.StatusBadRequest, errors.New("must enter a valid email address"))
		return
	}

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
}

func ChaosModeSignIn(c *gin.Context) {
	session := GetSession(c)

	if c.Request.Method == http.MethodGet {
		if !session.SignedIn() {
			c.HTML(http.StatusOK, "chaosSignIn.html.tmpl", nil)
			return
		}

		user, err := db.GetChaosUser(session.UserName)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		c.HTML(http.StatusOK, "chaosSignIn.html.tmpl", user)
		return
	}

	emailAddr := c.PostForm("email-address")
	if !localDevMode && !strings.Contains(emailAddr, "@") {
		// throw an error if it's not a valid email on prod
		c.AbortWithError(http.StatusBadRequest, errors.New("must enter a valid email address"))
		return
	}

	// get user by email somehow
	user, err := db.GetChaosUser(emailAddr)
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

	if user.TicketLimit < 1 {
		c.HTML(http.StatusOK, "ticketSalesClosed.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
		})
		return
	}

	c.HTML(http.StatusOK, "chaosSignIn.html.tmpl", user)
}

func ChaosModeCartHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	user, err := db.GetChaosUser(session.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if user.TicketLimit < 1 {
		c.HTML(http.StatusOK, "ticketSalesClosed.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
		})
		return
	}

	attendee, err := db.GetUser(session.UserName)
	if err == nil && attendee != nil {
		if attendee.OrderID != "" {
			order, err := db.GetOrder(attendee.OrderID)
			if err == nil && order != nil && order.PaymentStatus == "success" {
				c.Redirect(http.StatusFound, "/checkout-complete")
				return
			}
		}
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "hardLaunchCart.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
			"User":    user,
		})
		return
	}

	ticketType := c.PostForm("ticket-type")
	adultTix, _ := strconv.Atoi(c.PostForm("adult-tickets"))
	childTix, _ := strconv.Atoi(c.PostForm("child-tickets"))
	toddlerTix, _ := strconv.Atoi(c.PostForm("toddler-tickets"))
	donationAmount, _ := strconv.Atoi(c.PostForm("donation-amount"))

	if adultTix > user.TicketLimit {
		log.Errorf("user ticket limit exceeded %d", adultTix)
		ErrorFlash(c, fmt.Sprintf("You're limited to %d adult tickets", user.TicketLimit))
		return
	}

	totalTix := adultTix + childTix + toddlerTix
	dbTicketType := ""

	if adultTix > 0 {
		dbTicketType = "Adult"
	} else if childTix > 0 {
		dbTicketType = "Child"
	} else if toddlerTix > 0 {
		dbTicketType = "Toddler"
	}

	var admissionLevel string
	if ticketType == "cabin" {
		admissionLevel = "Cabin"
		cabinCap, err := db.GetConstant(fields.CabinCap)

		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		cabinSold, err := db.GetAggregation(fields.CabinSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if totalTix+cabinSold.Quantity > cabinCap.Value {
			log.Errorf("cabin ticket limit exceeded %d", totalTix+cabinSold.Quantity)
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many cabin tickets exceeds our cap! %d cabin tickets left.", cabinCap.Value-cabinSold.Quantity))
			return
		}

	} else if ticketType == "tent" {
		admissionLevel = "Tent"
	} else if ticketType == "sat" {
		admissionLevel = "Saturday Night"
	}

	if ticketType != "sat" {
		salesCap, err := db.GetConstant(fields.SalesCap)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		fullTixSold, err := db.GetAggregation(fields.FullSold)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if totalTix+fullTixSold.Quantity > salesCap.Value {
			log.Errorf("total ticket limit exceeded %d", totalTix+fullTixSold.Quantity)
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many tickets exceeds our cap! %d tickets left.", salesCap.Value-fullTixSold.Quantity))
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

		if totalTix+satSold.Quantity > satCap.Value {
			log.Errorf("saturday ticket limit exceeded %d", totalTix+satSold.Quantity)
			ErrorFlash(c, fmt.Sprintf("Sorry, buying that many tickets exceeds our Saturday night cap! %d Saturday tickets left.", satCap.Value-satSold.Quantity))
			return
		}
	}

	newUser := &db.User{
		AirtableID:         "",
		UserName:           user.UserName,
		TwitterName:        user.TwitterName,
		Name:               user.Name,
		Email:              user.Email,
		AdmissionLevel:     admissionLevel,
		TicketType:         dbTicketType,
		CheckedIn:          false,
		Barcode:            "",
		OrderNotes:         "",
		OrderID:            "",
		TicketID:           "",
		SponsorshipConfirm: false,
		Badge:              c.PostForm("badge-checkbox") == "on",
		Vegetarian:         c.PostForm("vegetarian") == "on",
		GlutenFree:         c.PostForm("glutenfree") == "on",
		LactoseIntolerant:  c.PostForm("lactose") == "on",
		FoodComments:       c.PostForm("comments"),
		DiscordName:        c.PostForm("discord-name"),
		TicketPath:         user.Phase,
	}

	if attendee != nil {
		newUser.OrderID = attendee.OrderID
		newUser.AirtableID = attendee.AirtableID
		err = newUser.UpdateUser()
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	} else {
		err = newUser.CreateUser()
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	params := url.Values{}
	params.Set("ticketType", ticketType)
	params.Set("adult", strconv.Itoa(adultTix))
	params.Set("child", strconv.Itoa(childTix))
	params.Set("toddler", strconv.Itoa(toddlerTix))
	params.Set("donation", strconv.Itoa(donationAmount))

	c.Redirect(http.StatusFound, "/checkout"+"?"+params.Encode())
}

func SignInRedirect(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	findUser(c, session.UserName, false)
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
	var items []db.Item
	for ind, element := range ticketIds {
		amt, _ := strconv.Atoi(c.Query(element))
		if amt > 0 {
			if ind < 3 {
				items = append(items, db.Item{
					Id:       element + "-" + ticketType,
					Quantity: amt,
					Amount:   0,
				})
			} else {
				items = append(items, db.Item{
					Id:       ticketIds[ind],
					Quantity: 1,
					Amount:   amt,
				})
			}
		}
	}

	// log.Debugf("%v", items)
	itemMap := struct {
		Items []db.Item `json:"items"`
	}{
		Items: items,
	}
	// log.Debugf("%v", itemMap)
	itemJson, err := json.Marshal(itemMap)
	if err != nil {
		log.Errorf("%v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	// log.Debugf(string(itemJson))

	c.HTML(http.StatusOK, "checkout.html.tmpl", gin.H{
		"User":      user,
		"Items":     string(itemJson),
		"OrderType": "Purchase",
		"UserType":  user.TicketPath,
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

	order, err := db.GetOrderByPaymentID(c.Query("payment_id"))
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.HTML(http.StatusOK, "purchaseComplete.html.tmpl", gin.H{
		"User":  user,
		"Order": order,
	})
}

func PurchaseFailedHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "purchaseFailed.html.tmpl", gin.H{})
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

	if order.PaymentStatus != "failed" {
		c.Redirect(http.StatusFound, "/vc2")
		return
	}

	user.OrderID = ""
	err = user.UpdateUser()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Redirect(http.StatusFound, "/vc2")
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

	needsOrder := !(user.AdmissionLevel == "Staff" || user.TicketPath == "Sponsorship" || user.TicketPath == fields.Volunteer || user.TicketPath == fields.TicketSwap || user.TicketPath == fields.Comped)

	order, err := db.GetOrder(user.OrderID)
	if needsOrder {
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		if order.PaymentStatus == "failed" {
			c.Redirect(http.StatusFound, "/checkout-failed")
			return
		} else if order.PaymentStatus == "processing" {
			c.Redirect(http.StatusFound, "/checkout-complete")
			return
		}
	}

	if c.Request.Method == http.MethodGet {
		c.HTML(http.StatusOK, "logistics2023.html.tmpl", gin.H{
			"flashes": GetFlashes(c),
			"User":    user,
			"Order":   order,
		})
		return
	}

	badge := c.PostForm("badge-checkbox") == "on"
	vegetarian := c.PostForm("vegetarian") == "on"
	glutenFree := c.PostForm("glutenfree") == "on"
	lactoseIntolerant := c.PostForm("lactose") == "on"
	foodComments := c.PostForm("comments")
	discordName := c.PostForm("discord-name")

	/*
		assistanceToCamp := c.PostForm("travel-from-airport")
		assistanceFromCamp := c.PostForm("assistance-from-camp") == "on"
		wrongCityRedirect := c.PostForm("wrong-city-redirect") == "Yes"
		rvCamper := c.PostForm("rv-camper")
		travelMethod := c.PostForm("travel-method")
		flyingInto := c.PostForm("flying-into")
		flightArrivalTime := c.PostForm("flight-arrival-time")
		vehicleArrivalTime := c.PostForm("vehicle-arrival-time")
		leavingFrom := c.PostForm("leaving-from")
		cityArrivalTime := c.PostForm("city-arrival-time")
		sleepingBagRentals, _ := strconv.Atoi(c.PostForm("sleeping-bag-rentals"))
		sheetRentals, _ := strconv.Atoi(c.PostForm("sheet-rentals"))
		pillowRentals, _ := strconv.Atoi(c.PostForm("pillow-rentals"))
		earlyArrival := c.PostForm("early-arrival")
	*/

	err = user.Set2023Logistics(badge, vegetarian, glutenFree, lactoseIntolerant, foodComments, discordName)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	SuccessFlash(c, "Saved!")

	c.Redirect(http.StatusFound, "/2023-logistics")
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

type DiscordResponse struct {
	UserFound bool `json:"user_found"`
}

func DiscordAuthenticator(c *gin.Context) {
	authToken := c.Request.Header[http.CanonicalHeaderKey("auth_token")]
	discordName := c.Query("discord_name")

	if authToken == nil {
		c.AbortWithError(http.StatusUnauthorized, errors.New("auth_token required"))
		return
	}

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		c.AbortWithError(http.StatusForbidden, errors.New("route disabled"))
		return
	}

	h := sha256.Sum256([]byte(hmacSecret))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(authToken[0])) != 1 {
		c.AbortWithError(http.StatusForbidden, errors.New("invalid auth_token"))
		return
	}

	user, err := db.GetUserByDiscord(discordName)
	if err != nil {
		// c.AbortWithError(http.StatusInternalServerError, err)
		c.JSON(http.StatusOK, DiscordResponse{UserFound: false})
	} else if user != nil {
		c.JSON(http.StatusOK, DiscordResponse{UserFound: true})
	} else {
		c.AbortWithError(http.StatusInternalServerError, errors.New("Unknown server error"))
	}
}

type AppEndpointResponse struct {
	TwitterName       string `json:"twitter_name,omitempty"`
	UserName          string `json:"user_name"`
	DiscordName       string `json:"discord_name"`
	TicketStatus      string `json:"ticket_status"`
	TicketType        string `json:"ticket_type"`
	TicketID          string `json:"ticket_id"`
	AccomodationType  string `json:"accomodation_type"`
	Cabin2022         string `json:"cabin_2022,omitempty"`
	Cabin2023         string `json:"cabin_2023,omitempty"`
	CabinNickname2023 string `json:"cabin_nickname_2023,omitempty"`
	CreatedAt         string `json:"created_at"`
	TentVillage2023   string `json:"tent_village_2023,omitempty"`
}

func AppEndpoint(c *gin.Context) {
	authToken := c.Request.Header[http.CanonicalHeaderKey("auth_token")]
	twitterName := c.Query("twitter_name")

	if authToken == nil {
		c.AbortWithError(http.StatusUnauthorized, errors.New("auth_token required"))
		return
	}

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		c.AbortWithError(http.StatusForbidden, errors.New("route disabled"))
		return
	}

	h := sha256.Sum256([]byte(hmacSecret))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(authToken[0])) != 1 {
		c.AbortWithError(http.StatusForbidden, errors.New("invalid auth_token"))
		return
	}

	user, _ := db.GetUser(twitterName)
	if user != nil {
		c.JSON(http.StatusOK, AppEndpointResponse{TwitterName: user.TwitterName, UserName: user.UserName, DiscordName: user.DiscordName, TicketStatus: "Active", TicketType: user.TicketType, TicketID: user.TicketID, AccomodationType: user.AdmissionLevel, Cabin2022: user.Cabin2022, CreatedAt: user.Created, Cabin2023: user.Cabin2023, CabinNickname2023: user.CabinNickname2023, TentVillage2023: user.TentVillage})
	} else {
		user, err := db.GetUserByField(fields.TwitterName, twitterName)
		if user != nil {
			c.JSON(http.StatusOK, AppEndpointResponse{TwitterName: user.TwitterName, UserName: user.UserName, DiscordName: user.DiscordName, TicketStatus: "Active", TicketType: user.TicketType, TicketID: user.TicketID, AccomodationType: user.AdmissionLevel, Cabin2022: user.Cabin2022, CreatedAt: user.Created, Cabin2023: user.Cabin2023, CabinNickname2023: user.CabinNickname2023, TentVillage2023: user.TentVillage})
			return
		}

		if err != nil {
			// c.AbortWithError(http.StatusInternalServerError, err)
			if err == errors.New("You're not on the guest list! Most likely we spelled your Twitter handle wrong.") {
				c.JSON(http.StatusNotFound, nil)
			} else {
				c.AbortWithError(http.StatusInternalServerError, err)
			}
		} else {
			c.AbortWithError(http.StatusInternalServerError, errors.New("Unknown server error"))
		}
	}
}

func UserByDiscordEndpoint(c *gin.Context) {
	authToken := c.Request.Header[http.CanonicalHeaderKey("auth_token")]
	discordName := c.Query("discord_name")

	if authToken == nil {
		c.AbortWithError(http.StatusUnauthorized, errors.New("auth_token required"))
		return
	}

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		c.AbortWithError(http.StatusForbidden, errors.New("route disabled"))
		return
	}

	h := sha256.Sum256([]byte(hmacSecret))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(authToken[0])) != 1 {
		c.AbortWithError(http.StatusForbidden, errors.New("invalid auth_token"))
		return
	}

	user, err := db.GetUserByDiscord(discordName)
	if err != nil {
		// c.AbortWithError(http.StatusInternalServerError, err)
		if err == db.ErrNoRecords {
			c.JSON(http.StatusNotFound, nil)
		} else {
			c.AbortWithError(http.StatusInternalServerError, err)
		}
	} else if user != nil {
		c.JSON(http.StatusOK, AppEndpointResponse{TwitterName: user.TwitterName, UserName: user.UserName, DiscordName: user.DiscordName, TicketStatus: "Active", TicketType: user.TicketType, TicketID: user.TicketID, AccomodationType: user.AdmissionLevel, Cabin2022: user.Cabin2022, CreatedAt: user.Created, Cabin2023: user.Cabin2023, CabinNickname2023: user.CabinNickname2023, TentVillage2023: user.TentVillage})
	} else {
		c.AbortWithError(http.StatusInternalServerError, errors.New("Unknown server error"))
	}
}

// gets all attendees and returns them in an array
func GetAttendeesEndpoint(c *gin.Context) {
	authToken := c.Request.Header[http.CanonicalHeaderKey("auth_token")]

	if authToken == nil {
		c.AbortWithError(http.StatusUnauthorized, errors.New("auth_token required"))
		return
	}

	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		c.AbortWithError(http.StatusForbidden, errors.New("route disabled"))
		return
	}

	h := sha256.Sum256([]byte(hmacSecret))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(authToken[0])) != 1 {
		c.AbortWithError(http.StatusForbidden, errors.New("invalid auth_token"))
		return
	}

	attendees, err := db.GetAttendees()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, attendees)
}
