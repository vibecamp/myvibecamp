package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
)

func InfoHandler(c *gin.Context) {
	session := GetSession(c)

	if !session.SignedIn() {
		c.HTML(http.StatusOK, "index.html.tmpl", nil)
		return
	}

	airtableApiKey := os.Getenv("AIRTABLE_API_KEY")
	if airtableApiKey == "" {
		log.Errorf("no airtable api key")
		os.Exit(1)
	}

	client := airtable.NewClient(airtableApiKey)
	table := client.GetTable(os.Getenv("AIRTABLE_DB"), "Attendees")
	records, err := table.GetRecords().
		//FromView("view_1").
		//WithFilterFormula("AND({Field1}='value_1',NOT({Field2}='value_2'))").
		WithFilterFormula(fmt.Sprintf("OR({Twitter Name}='%s',{Twitter Name}='@%s')", session.TwitterName, session.TwitterName)).
		//WithSort(sortQuery1, sortQuery2).
		ReturnFields("Ticket ID", "Twitter Name", "Cabin").
		InStringFormat("US/Eastern", "en").
		Do()
	if err != nil {
		panic(err)
	}

	if records == nil {
		c.String(http.StatusBadRequest, "you're not on the list")
		c.Abort()
		return
	} else if len(records.Records) != 1 {
		c.String(http.StatusBadRequest, "you're on the list more than once")
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
			c.String(http.StatusInternalServerError, "error getting cabin records")
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
