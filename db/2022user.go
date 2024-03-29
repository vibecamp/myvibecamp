package db

import (
	"encoding/gob"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

type OldUser struct {
	TwitterName       string
	TwitterNameClean  string
	Name              string
	AdmissionLevel    string
	Cabin             string
	CabinNumber       string
	TicketGroup       string
	CheckedIn         bool
	Barcode           string
	OrderNotes        string
	Badge             string
	TransportTo       string
	TransportFrom     string
	BeddingRental     string
	BeddingPaid       bool
	DepartureTime     string
	ArrivalTime       string
	BusToCamp         string
	BusToAUS          string
	Vegetarian        bool
	GlutenFree        bool
	LactoseIntolerant bool
	FoodComments      string
	POAP              string

	AirtableID string
}

func init() {
	gob.Register(OldUser{})
}

func GetOldUser(twitterName string) (*OldUser, error) {
	cleanName := strings.ToLower(twitterName)

	user, err := getOldUserByField(fields.TwitterNameClean, cleanName)
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

func getOldUserByField(field, value string) (*OldUser, error) {
	response, err := query(defaultTable, field, value) // get all fields
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(response.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	rec := response.Records[0]

	busToCamp := toStr(rec.Fields[fields.BusToCamp])
	if busToCamp != "" {
		busToCamp = strings.Split(busToCamp, " ")[1]
	}
	busToAUS := toStr(rec.Fields[fields.BusToAUS])
	if busToAUS != "" {
		busToAUS = strings.Split(busToAUS, " ")[1]
	}

	u := &OldUser{
		AirtableID:        rec.ID,
		TwitterName:       toStr(rec.Fields[fields.TwitterName]),
		TwitterNameClean:  toStr(rec.Fields[fields.TwitterNameClean]),
		Name:              toStr(rec.Fields[fields.Name]),
		AdmissionLevel:    toStr(rec.Fields[fields.AdmissionLevel]),
		Cabin:             toStr(rec.Fields[fields.Cabin]),
		CabinNumber:       toStr(rec.Fields[fields.CabinNumber]),
		TicketGroup:       toStr(rec.Fields[fields.TicketGroup]),
		CheckedIn:         rec.Fields[fields.CheckedIn] == checked,
		Barcode:           toStr(rec.Fields[fields.Barcode]),
		OrderNotes:        toStr(rec.Fields[fields.OrderNotes]),
		Badge:             toStr(rec.Fields[fields.Badge]),
		TransportTo:       toStr(rec.Fields[fields.TransportTo]),
		TransportFrom:     toStr(rec.Fields[fields.TransportFrom]),
		BeddingRental:     toStr(rec.Fields[fields.BeddingRental]),
		BeddingPaid:       rec.Fields[fields.BeddingPaid] == checked,
		DepartureTime:     toStr(rec.Fields[fields.DepartureTime]),
		ArrivalTime:       toStr(rec.Fields[fields.ArrivalTime]),
		BusToCamp:         busToCamp,
		BusToAUS:          busToAUS,
		Vegetarian:        rec.Fields[fields.Vegetarian] == checked,
		GlutenFree:        rec.Fields[fields.GlutenFree] == checked,
		LactoseIntolerant: rec.Fields[fields.LactoseIntolerant] == checked,
		FoodComments:      toStr(rec.Fields[fields.FoodComments]),
		POAP:              toStr(rec.Fields[fields.POAP]),
	}

	return u, nil
}

func MigrateData() error {
	offset := ""

	for {
		response, err := defaultTable.GetRecords().
			WithOffset(offset).
			ReturnFields(fields.TwitterNameClean, fields.Cabin, fields.Email).
			InStringFormat("US/Eastern", "en").
			Do()

		if err != nil {
			log.Errorf("%+v", err)
			return err
		}

		for _, r := range response.Records {
			name := toStr(r.Fields[fields.TwitterNameClean])
			if name == "" {
				name = toStr(r.Fields[fields.Email])
			}
			u, _ := GetUser(name)

			if u != nil {
				r := &airtable.Records{
					Records: []*airtable.Record{{
						ID: u.AirtableID,
						Fields: map[string]interface{}{
							fields.Cabin2022: toStr(r.Fields[fields.Cabin]),
						},
					}},
				}

				_, err := attendeesTable.UpdateRecordsPartial(r)
				if err != nil {
					log.Errorf("%+v", err)
				}
			} else {
				slu, _ := GetSoftLaunchUser(name)
				if slu != nil {
					r := &airtable.Records{
						Records: []*airtable.Record{{
							ID: slu.AirtableID,
							Fields: map[string]interface{}{
								fields.Cabin2022: toStr(r.Fields[fields.Cabin]),
							},
						}},
					}

					_, err := softLaunchTable.UpdateRecordsPartial(r)
					if err != nil {
						log.Errorf("%+v", err)
					}
				}
			}

			time.Sleep(3 * time.Second)
		}

		if response.Offset == "" {
			break
		}
		offset = response.Offset
	}

	return nil
}
