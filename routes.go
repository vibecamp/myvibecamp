package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/mehanizm/airtable"
)

func InfoHandler(c *gin.Context) {
	session := GetSession(c)

	if !session.SignedIn() {
		c.HTML(http.StatusOK, "index.html.tmpl", nil)
		return
	}

	client := airtable.NewClient(os.Getenv("AIRTABLE_API_KEY"))
	table := client.GetTable(os.Getenv("AIRTABLE_DB"), "Attendees")
	records, err := table.GetRecords().
		//FromView("view_1").
		//WithFilterFormula("AND({Field1}='value_1',NOT({Field2}='value_2'))").
		WithFilterFormula(fmt.Sprintf("{twitter clean}='%s'", strings.ToLower(session.TwitterName))).
		//WithSort(sortQuery1, sortQuery2).
		ReturnFields("Ticket ID", "Twitter Name", "Cabin").
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
			ReturnFields("Twitter Name").
			InStringFormat("US/Eastern", "en").
			Do()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "getting cabin records"))
			c.Abort()
			return
		}
		for _, c := range cRecs.Records {
			cabinMates = append(cabinMates, fmt.Sprintf("%s", c.Fields["Twitter Name"]))
		}
	}

	c.HTML(http.StatusOK, "info.html.tmpl", struct {
		Name       string
		Cabin      string
		Cabinmates []string
	}{
		Name:       session.TwitterName,
		Cabin:      fmt.Sprintf("%s", rec.Fields["Cabin"]),
		Cabinmates: cabinMates,
	})
}
