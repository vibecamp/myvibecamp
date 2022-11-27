package db

import (
	"bytes"
	"encoding/gob"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

type SoftLaunchUser struct {
	UserName          string
	Name              string
	TwitterName       string
	Email             string
	TicketLimit       int
	Badge             bool
	POAP              string
	Vegetarian        bool
	GlutenFree        bool
	LactoseIntolerant bool
	AirtableID        string
}

func GetSoftLaunchUser(userName string) (*SoftLaunchUser, error) {
	cleanName := strings.ToLower(userName)
	if defaultCache != nil {
		if u, found := defaultCache.Get("sl-" + cleanName); found {
			log.Trace("user cache hit")
			var user SoftLaunchUser
			err := gob.NewDecoder(bytes.NewBuffer(u.([]byte))).Decode(&user)
			if err != nil {
				return nil, errors.Wrap(err, "cache hit")
			}
			return &user, nil
		}
	}

	user, err := getSoftLaunchUserByField(fields.UserName, cleanName)
	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("You're not on the guest list! Most likely we spelled your Twitter handle wrong.")
		} else if errors.Is(err, ErrManyRecords) {
			err = errors.New("You're on the list multiple times. We probably screwed something up 😰")
		}
		return nil, err
	} else if user == nil {
		return nil, errors.New("no user found, but no error from db 🤔")
	}

	return user, nil
}

func getSoftLaunchUserByField(field, value string) (*SoftLaunchUser, error) {
	response, err := query(softLaunchUsersTable, field, value) // get all fields
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(response.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	rec := response.Records[0]
	ticketLimit, _ := strconv.Atoi(rec.Fields[fields.TicketLimit].(string))

	u := &SoftLaunchUser{
		AirtableID:        rec.ID,
		UserName:          toStr(rec.Fields[fields.UserName]),
		TwitterName:       toStr(rec.Fields[fields.TwitterName]),
		Name:              toStr(rec.Fields[fields.Name]),
		Email:             toStr(rec.Fields[fields.Email]),
		POAP:              toStr(rec.Fields[fields.POAP]),
		Badge:             toStr(rec.Fields[fields.Badge]) == "yes",
		TicketLimit:       ticketLimit,
		Vegetarian:        rec.Fields[fields.Vegetarian] == checked,
		GlutenFree:        rec.Fields[fields.GlutenFree] == checked,
		LactoseIntolerant: rec.Fields[fields.LactoseIntolerant] == checked,
	}

	if defaultCache != nil {
		var b bytes.Buffer
		err := gob.NewEncoder(&b).Encode(*u)
		if err != nil {
			return nil, errors.Wrap(err, "cache save")
		}
		defaultCache.Set(u.cacheKey(), b.Bytes(), 0)
	}

	return u, nil
}
