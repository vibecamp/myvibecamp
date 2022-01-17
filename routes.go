package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kurrik/oauth1a"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
)

func SignInHandler(c *gin.Context) {
	userConfig := &oauth1a.UserConfig{}
	err := userConfig.GetRequestToken(context.Background(), service, http.DefaultClient)
	if err != nil {
		log.Debugf("Could not get request token: %v", err)
		c.String(http.StatusInternalServerError, "Problem getting the request token")
		c.Abort()
		return
	}

	url, err := userConfig.GetAuthorizeURL(service)
	if err != nil {
		log.Debugf("Could not get authorization URL: %v", err)
		c.String(http.StatusInternalServerError, "Problem getting the authorization URL")
		c.Abort()
		return
	}

	sessionID := NewSessionID()
	log.Debugf("Starting session %v\n", sessionID)

	var conf bytes.Buffer
	enc := gob.NewEncoder(&conf)
	err = enc.Encode(userConfig)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "encoding oauth config"))
		return
	}

	session := sessions.Default(c)
	session.Set("oauth_config", conf.Bytes())
	session.Save()

	log.Debugf("Redirecting user to %v\n", url)
	c.Redirect(http.StatusFound, url)
}

func CallbackHandler(c *gin.Context) {
	log.Tracef("Callback hit") //. %v current sessions.\n", len(sessions))

	session := sessions.Default(c)
	userConfigInt := session.Get("oauth_config")
	if userConfigInt == nil {
		log.Tracef("No user config in session")
		c.String(http.StatusBadRequest, "error: no session found")
		c.Abort()
		return
	}

	var userConfig *oauth1a.UserConfig
	enc := gob.NewDecoder(bytes.NewBuffer(userConfigInt.([]byte)))
	err := enc.Decode(&userConfig)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "decoding oauth config from session"))
		return
	}

	token, verifier, err := userConfig.ParseAuthorize(c.Request, service)
	if err != nil {
		log.Tracef("Could not parse authorization: %v", err)
		c.String(http.StatusInternalServerError, "error: could not parse authorization")
		c.Abort()
		return
	}

	err = userConfig.GetAccessToken(context.Background(), token, verifier, service, http.DefaultClient)
	if err != nil {
		log.Tracef("Error getting access token: %v", err)
		c.String(http.StatusInternalServerError, "error: could not get access token")
		c.Abort()
		return
	}

	session.Set("twitterName", userConfig.AccessValues.Get("screen_name"))
	session.Set("twitterID", userConfig.AccessValues.Get("user_id"))
	session.Save()

	//session.key = userConfig.AccessTokenKey
	//session.secret = userConfig.AccessTokenSecret
	//session.twitterName = userConfig.AccessValues.Get("screen_name")
	//session.twitterID = userConfig.AccessValues.Get("user_id")
	c.Redirect(http.StatusFound, "/info")
}

func InfoHandler(c *gin.Context) {
	session := sessions.Default(c)
	twitterNameInt := session.Get("twitterName")
	if twitterNameInt == nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	twitterName := twitterNameInt.(string)

	if twitterName == "" {
		session.Clear()
		c.Redirect(http.StatusFound, "/?error=Invalid+session")
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
		WithFilterFormula(fmt.Sprintf("OR({Twitter Name}='%s',{Twitter Name}='@%s')", twitterName, twitterName)).
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
		Name:       twitterName,
		Cabin:      fmt.Sprintf("%s", rec.Fields["Cabin"]),
		Cabinmates: cabinMates,
	})
}
