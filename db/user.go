package db

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/lyoshenka/vibedata/fields"
	"github.com/mehanizm/airtable"
)

const checked = "checked"

var defaultTable *airtable.Table

func Init(apiKey, baseID, tableName string) {
	defaultTable = airtable.NewClient(apiKey).GetTable(baseID, tableName)
}

var ErrNoRecords = fmt.Errorf("no records found")
var ErrManyRecords = fmt.Errorf("multiple records for value")

type User struct {
	TwitterName      string
	TwitterNameClean string
	Cabin            string
	TicketGroup      string
	CheckedIn        bool
	Barcode          string
	Badge            string
	TransportTo      string
	TransportFrom    string
	BeddingRental    string
	BeddingPaid      bool
	DepartureTime    string
	ArrivalTime      string

	airtableID string
}

func GetUserFromBarcode(barcode string) (*User, error) {
	return getUserByField(fields.Barcode, barcode)
}

func GetUser(twitterName string) (*User, error) {
	user, err := getUserByField(fields.TwitterNameClean, strings.ToLower(twitterName))

	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("You're not on the guest list! Most likely we spelled your Twitter handle wrong.")
		} else if errors.Is(err, ErrManyRecords) {
			err = errors.New("You're on the list multiple times. We probably screwed something up 😰")
		}
	}

	return user, err
}

func getUserByField(field, value string) (*User, error) {
	//rec, err := query(field, value,
	//	fields.TwitterName, fields.TwitterNameClean,
	//	fields.Cabin,
	//	fields.TicketGroup, fields.CheckedIn, fields.Barcode,
	//	fields.Badge,
	//)
	rec, err := query(field, value) // get all fields
	if err != nil {
		return nil, err
	}

	return &User{
		airtableID:       rec.ID,
		TwitterName:      toStr(rec.Fields[fields.TwitterName]),
		TwitterNameClean: toStr(rec.Fields[fields.TwitterNameClean]),
		Cabin:            toStr(rec.Fields[fields.Cabin]),
		TicketGroup:      toStr(rec.Fields[fields.TicketGroup]),
		CheckedIn:        rec.Fields[fields.CheckedIn] == checked,
		Barcode:          toStr(rec.Fields[fields.Barcode]),
		Badge:            toStr(rec.Fields[fields.Badge]),
		TransportTo:      toStr(rec.Fields[fields.TransportTo]),
		TransportFrom:    toStr(rec.Fields[fields.TransportFrom]),
		BeddingRental:    toStr(rec.Fields[fields.BeddingRental]),
		BeddingPaid:      rec.Fields[fields.BeddingPaid] == checked,
		DepartureTime:    toStr(rec.Fields[fields.DepartureTime]),
		ArrivalTime:      toStr(rec.Fields[fields.ArrivalTime]),
	}, nil
}

func query(field, value string, returnFields ...string) (*airtable.Record, error) {
	records, err := defaultTable.GetRecords().
		//FromView("view_1").
		//WithFilterFormula("AND({Field1}='value_1',NOT({Field2}='value_2'))").
		WithFilterFormula(fmt.Sprintf("{%s}='%s'", field, value)).
		//WithSort(sortQuery1, sortQuery2).
		ReturnFields(returnFields...).
		InStringFormat("US/Eastern", "en").
		Do()
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	if records == nil || len(records.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(records.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	return records.Records[0], nil
}

func (u *User) SetBadge(badgeChoice string) error {
	if badgeChoice != "yes" && badgeChoice != "no" {
		return errors.Newf("invalid badge choice: '%s'", badgeChoice)
	}

	u.Badge = badgeChoice

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.airtableID,
			Fields: map[string]interface{}{
				fields.Badge: u.Badge,
			},
		}},
	}

	_, err := defaultTable.UpdateRecordsPartial(r)
	return errors.Wrap(err, "setting badge")
}

func (u *User) GetCabinMates() ([]string, error) {
	if u.Cabin == "" {
		return nil, nil
	}

	var cabinMates []string
	response, err := defaultTable.GetRecords().
		WithFilterFormula(fmt.Sprintf("{%s}='%s'", fields.Cabin, u.Cabin)).
		ReturnFields(fields.TwitterName, fields.TwitterNameClean).
		InStringFormat("US/Eastern", "en").
		Do()
	if err != nil {
		return nil, errors.Wrap(err, "getting cabin records")
	}

	for _, c := range response.Records {
		if c.Fields[fields.TwitterNameClean] != u.TwitterNameClean {
			cabinMates = append(cabinMates, toStr(c.Fields[fields.TwitterName]))
		}
	}

	return cabinMates, nil
}

type TicketGroupEntry struct {
	TwitterName string
	CheckedIn   bool
}

func (u *User) GetTicketGroup() ([]TicketGroupEntry, error) {
	if u.TicketGroup == "" {
		return []TicketGroupEntry{{u.TwitterName, u.CheckedIn}}, nil
	}

	var ticketGroup []TicketGroupEntry
	response, err := defaultTable.GetRecords().
		WithFilterFormula(fmt.Sprintf("{%s}='%s'", fields.TicketGroup, u.TwitterName)).
		ReturnFields(fields.TwitterName, fields.CheckedIn).
		InStringFormat("US/Eastern", "en").
		Do()
	if err != nil {
		return nil, errors.Wrap(err, "getting cabin records")
	}

	for _, c := range response.Records {
		ticketGroup = append(ticketGroup, TicketGroupEntry{
			TwitterName: toStr(c.Fields[fields.TwitterName]),
			CheckedIn:   c.Fields[fields.CheckedIn] == checked,
		})
	}

	return ticketGroup, nil
}

func toStr(i interface{}) string {
	if i == nil {
		return ""
	}
	return i.(string)
}
