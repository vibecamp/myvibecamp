package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/mehanizm/airtable"
	"github.com/skip2/go-qrcode"
)

func InfoHandler(c *gin.Context) {
	session := GetSession(c)

	if !session.SignedIn() {
		c.HTML(http.StatusOK, "index.html.tmpl", nil)
		return
	}

	cleanTwitterName := strings.ToLower(session.TwitterName)

	client := airtable.NewClient(os.Getenv("AIRTABLE_API_KEY"))
	table := client.GetTable(os.Getenv("AIRTABLE_BASE_ID"), os.Getenv("AIRTABLE_TABLE_NAME"))
	records, err := table.GetRecords().
		//FromView("view_1").
		//WithFilterFormula("AND({Field1}='value_1',NOT({Field2}='value_2'))").
		WithFilterFormula(fmt.Sprintf("{twitter clean}='%s'", cleanTwitterName)).
		//WithSort(sortQuery1, sortQuery2).
		ReturnFields("Ticket ID", "Twitter Name", "Cabin", "Barcode").
		InStringFormat("US/Eastern", "en").
		Do()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, ""))
		return
	}

	if records == nil || len(records.Records) == 0 {
		c.HTML(http.StatusBadRequest, "error.html.tmpl", "You're not on the guest list.")
		c.Abort()
		return
	} else if len(records.Records) != 1 {
		c.HTML(http.StatusBadRequest, "error.html.tmpl", "You're on the guest list more than once. We probably screwed something up.")
		c.Abort()
		return
	}

	rec := records.Records[0]
	var cabinMates []string

	if rec.Fields["Cabin"] != "" {
		cRecs, err := table.GetRecords().
			WithFilterFormula(fmt.Sprintf("{Cabin}='%s'", rec.Fields["Cabin"])).
			ReturnFields("Twitter Name", "twitter clean").
			InStringFormat("US/Eastern", "en").
			Do()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "getting cabin records"))
			return
		}
		for _, c := range cRecs.Records {
			if c.Fields["twitter clean"] != cleanTwitterName {
				cabinMates = append(cabinMates, fmt.Sprintf("%s", c.Fields["Twitter Name"]))
			}
		}
	}

	qr, err := qrcode.Encode(rec.Fields["Barcode"].(string), qrcode.Medium, 256)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "generating qr code"))
		return
	}

	var cabin string
	if rec.Fields["Cabin"] != nil {
		cabin = strings.TrimSpace(fmt.Sprintf("%s", rec.Fields["Cabin"]))
	}

	c.HTML(http.StatusOK, "info.html.tmpl", struct {
		Name       string
		Cabin      string
		Cabinmates []string
		QR         string
	}{
		Name:       session.TwitterName,
		Cabin:      cabin,
		Cabinmates: cabinMates,
		QR:         base64.StdEncoding.EncodeToString(qr),
	})
}
