package db

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/lyoshenka/vibedata/fields"
	"github.com/mehanizm/airtable"
)

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
}

func GetUserFromBarcode(barcode string) (*User, error) {
	return getUserByField(fields.Barcode, barcode)
}

func GetUser(twitterName string) (*User, error) {
	user, err := getUserByField(fields.TwitterNameClean, twitterName)

	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("You're not on the guest list!")
		} else if errors.Is(err, ErrManyRecords) {
			err = errors.New("You're on the list multiple times. We probably screwed something up 😰")
		}
	}

	return user, err
}

func getUserByField(field, value string) (*User, error) {
	rec, err := query(field, value,
		fields.TwitterName, fields.TwitterNameClean,
		fields.Cabin,
		fields.TicketGroup, fields.CheckedIn, fields.Barcode,
	)
	if err != nil {
		return nil, err
	}

	return &User{
		TwitterName:      toStr(rec.Fields[fields.TwitterName]),
		TwitterNameClean: toStr(rec.Fields[fields.TwitterNameClean]),
		Cabin:            toStr(rec.Fields[fields.Cabin]),
		TicketGroup:      toStr(rec.Fields[fields.TicketGroup]),
		CheckedIn:        rec.Fields[fields.CheckedIn] == "checked",
		Barcode:          toStr(rec.Fields[fields.Barcode]),
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
			CheckedIn:   c.Fields[fields.CheckedIn] == "checked",
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
