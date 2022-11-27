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

type ChaosModeUser struct {
	UserName    string
	Name        string
	TwitterName string
	Email       string
	TicketLimit int
	Phase       string

	AirtableID string
}

func GetChaosUser(userName string) (*ChaosModeUser, error) {
	cleanName := strings.ToLower(userName)
	if defaultCache != nil {
		if u, found := defaultCache.Get("cm-" + cleanName); found {
			log.Trace("user cache hit")
			var user ChaosModeUser
			err := gob.NewDecoder(bytes.NewBuffer(u.([]byte))).Decode(&user)
			if err != nil {
				return nil, errors.Wrap(err, "cache hit")
			}
			return &user, nil
		}
	}

	user, err := getChaosUserByField(fields.UserName, cleanName)
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

func getChaosUserByField(field, value string) (*ChaosModeUser, error) {
	response, err := query(chaosModeUsersTable, field, value) // get all fields
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

	u := &ChaosModeUser{
		AirtableID:  rec.ID,
		UserName:    toStr(rec.Fields[fields.UserName]),
		TwitterName: toStr(rec.Fields[fields.TwitterName]),
		Name:        toStr(rec.Fields[fields.Name]),
		Email:       toStr(rec.Fields[fields.Email]),
		Phase:       toStr(rec.Fields[fields.Phase]),
		TicketLimit: ticketLimit,
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
