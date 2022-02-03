package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/lyoshenka/vibedata/db"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
)

func InfoHandler(c *gin.Context) {
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

	cabinMates, err := user.GetCabinMates()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	//qr, err := qrcode.Encode(rec.Fields["Barcode"].(string), qrcode.Medium, 256)
	//if err != nil {
	//	c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "generating qr code"))
	//	return
	//}

	c.HTML(http.StatusOK, "info.html.tmpl", struct {
		Name       string
		Cabin      string
		Cabinmates []string
		QR         string
	}{
		Name:       session.TwitterName,
		Cabin:      strings.TrimSpace(user.Cabin),
		Cabinmates: cabinMates,
		//QR:         base64.StdEncoding.EncodeToString(qr),
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

	ticketDomain := ""
	if !localDevMode {
		ticketDomain = "https://my.vibecamp.xyz"
	}

	qr, err := qrcode.Encode(fmt.Sprintf(`%s/checkin/%s`, ticketDomain, user.Barcode), qrcode.Medium, 256)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "generating qr code"))
		return
	}

	c.HTML(http.StatusOK, "ticket.html.tmpl", struct {
		Name        string
		QR          string
		TicketGroup []db.TicketGroupEntry
	}{
		Name:        session.TwitterName,
		QR:          base64.StdEncoding.EncodeToString(qr),
		TicketGroup: ticketGroup,
	})
}

func CheckinHandler(c *gin.Context) {
	session := GetSession(c)
	if !session.SignedIn() {
		c.Redirect(http.StatusFound, "/")
		return
	} else if !session.HasCheckinPermission() {
		c.AbortWithError(http.StatusForbidden, errors.New("This page is for staff only"))
		return
	}

	barcode := c.Param("barcode")
	user, err := db.GetUserFromBarcode(barcode)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ticketGroup, err := user.GetTicketGroup()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "checkin.html.tmpl", struct {
		TicketGroup []db.TicketGroupEntry
	}{
		TicketGroup: ticketGroup,
	})
}
