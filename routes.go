package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/kurrik/oauth1a"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
)

func SignInHandler(w http.ResponseWriter, r *http.Request) {
	var (
		url       string
		err       error
		sessionID string
	)
	userConfig := &oauth1a.UserConfig{}

	if err = userConfig.GetRequestToken(context.Background(), service, http.DefaultClient); err != nil {
		log.Debugf("Could not get request token: %v", err)
		http.Error(w, "Problem getting the request token", 500)
		return
	}
	if url, err = userConfig.GetAuthorizeURL(service); err != nil {
		log.Debugf("Could not get authorization URL: %v", err)
		http.Error(w, "Problem getting the authorization URL", 500)
		return
	}
	log.Debugf("Redirecting user to %v\n", url)
	sessionID = NewSessionID()
	log.Debugf("Starting session %v\n", sessionID)
	sessions[sessionID] = &Session{oauth: userConfig}
	http.SetCookie(w, SessionStartCookie(sessionID))
	http.Redirect(w, r, url, 302)
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		token     string
		verifier  string
		sessionID string
		session   *Session
		ok        bool
	)

	log.Tracef("Callback hit. %v current sessions.\n", len(sessions))
	if sessionID, err = GetSessionID(r); err != nil {
		log.Tracef("Got a callback with no session id: %v\n", err)
		http.Error(w, "No session found", 400)
		return
	}
	if session, ok = sessions[sessionID]; !ok {
		log.Tracef("Could not find user config in sesions storage.")
		http.Error(w, "Invalid session", 400)
		return
	}
	if token, verifier, err = session.oauth.ParseAuthorize(r, service); err != nil {
		log.Tracef("Could not parse authorization: %v", err)
		http.Error(w, "Problem parsing authorization", 500)
		return
	}
	if err = session.oauth.GetAccessToken(context.Background(), token, verifier, service, http.DefaultClient); err != nil {
		log.Tracef("Error getting access token: %v", err)
		http.Error(w, "Problem getting an access token", 500)
		return
	}

	//log.Printf("Ending session %v.\n", sessionID)
	//delete(sessions, sessionID)
	//http.SetCookie(rw, SessionEndCookie())

	session.key = session.oauth.AccessTokenKey
	session.secret = session.oauth.AccessTokenSecret
	session.twitterName = session.oauth.AccessValues.Get("screen_name")
	session.twitterID = session.oauth.AccessValues.Get("user_id")
	http.Redirect(w, r, "/info", 302)
}

func InfoHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		sessionID string
		session   *Session
		ok        bool
	)

	if sessionID, err = GetSessionID(r); err != nil {
		http.Redirect(w, r, "/?error=No+session+found", 302)
		return
	}
	if session, ok = sessions[sessionID]; !ok {
		http.Redirect(w, r, "/?error=Invalid+session", 302)
		return
	}
	if session.twitterName == "" {
		delete(sessions, sessionID)
		http.SetCookie(w, SessionEndCookie())
		http.Redirect(w, r, "/?error=Invalid+session", 302)
		return
	}

	w.Header().Set("Content-Type", "text/html;charset=utf-8")

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
		WithFilterFormula(fmt.Sprintf("OR({Twitter Name}='%s',{Twitter Name}='@%s')", session.twitterName, session.twitterName)).
		//WithSort(sortQuery1, sortQuery2).
		ReturnFields("Ticket ID", "Twitter Name", "Cabin").
		InStringFormat("US/Eastern", "en").
		Do()
	if err != nil {
		panic(err)
	}

	if records == nil {
		fmt.Fprintf(w, "ERROR: records is nil")
		return
	} else if len(records.Records) != 1 {
		fmt.Fprintf(w, "ERROR: expected one record, found %d", len(records.Records))
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
			panic(err)
		}
		for _, c := range cRecs.Records {
			cabinMates = append(cabinMates, fmt.Sprintf("%s", c.Fields["Twitter Name"]))
		}
	}

	tmpl.ExecuteTemplate(w, "info.html.tmpl", struct {
		Name       string
		Cabin      string
		Cabinmates []string
	}{
		Name:       session.twitterName,
		Cabin:      fmt.Sprintf("%s", rec.Fields["Cabin"]),
		Cabinmates: cabinMates,
	})
}
