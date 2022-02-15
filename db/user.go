package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/lyoshenka/vibedata/fields"
	"github.com/mehanizm/airtable"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
)

const checked = "checked"

var defaultTable *airtable.Table
var defaultCache *cache.Cache

func Init(apiKey, baseID, tableName string, cache *cache.Cache) {
	defaultTable = airtable.NewClient(apiKey).GetTable(baseID, tableName)
	defaultCache = cache
}

var ErrNoRecords = fmt.Errorf("no records found")
var ErrManyRecords = fmt.Errorf("multiple records for value")

type User struct {
	TwitterName       string
	TwitterNameClean  string
	Cabin             string
	TicketGroup       string
	CheckedIn         bool
	Barcode           string
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

	AirtableID string
}

func init() {
	gob.Register(User{})
}

func GetUserFromBarcode(barcode string) (*User, error) {
	return getUserByField(fields.Barcode, barcode)
}

func GetUser(twitterName string) (*User, error) {
	cleanName := strings.ToLower(twitterName)
	if defaultCache != nil {
		if u, found := defaultCache.Get(cleanName); found {
			log.Trace("user cache hit")
			var user User
			err := gob.NewDecoder(bytes.NewBuffer(u.([]byte))).Decode(&user)
			if err != nil {
				return nil, errors.Wrap(err, "cache hit")
			}
			return &user, nil
		}
	}

	user, err := getUserByField(fields.TwitterNameClean, cleanName)
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

	if defaultCache != nil {
		var b bytes.Buffer
		err := gob.NewEncoder(&b).Encode(*user)
		if err != nil {
			return nil, errors.Wrap(err, "cache save")
		}
		defaultCache.Set(cleanName, b.Bytes(), 0)
	}

	return user, nil
}

func getUserByField(field, value string) (*User, error) {
	//rec, err := query(field, value,
	//	fields.TwitterName, fields.TwitterNameClean,
	//	fields.Cabin,
	//	fields.TicketGroup, fields.CheckedIn, fields.Barcode,
	//	fields.Badge,
	//)
	response, err := query(field, value) // get all fields
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

	return &User{
		AirtableID:        rec.ID,
		TwitterName:       toStr(rec.Fields[fields.TwitterName]),
		TwitterNameClean:  toStr(rec.Fields[fields.TwitterNameClean]),
		Cabin:             toStr(rec.Fields[fields.Cabin]),
		TicketGroup:       toStr(rec.Fields[fields.TicketGroup]),
		CheckedIn:         rec.Fields[fields.CheckedIn] == checked,
		Barcode:           toStr(rec.Fields[fields.Barcode]),
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
	}, nil
}

func query(field, value string, returnFields ...string) (*airtable.Records, error) {
	filterFormula := fmt.Sprintf(`{%s}="%s"`, field, strings.ReplaceAll(value, `"`, `\"`))
	log.Debugf(`airtable query: %s `, filterFormula)
	records, err := defaultTable.GetRecords().
		//FromView("view_1").
		WithFilterFormula(filterFormula).
		//WithSort(sortQuery1, sortQuery2).
		ReturnFields(returnFields...).
		InStringFormat("US/Eastern", "en").
		Do()
	return records, errors.Wrap(err, "")
}

func (u *User) SetBadge(badgeChoice string) error {
	if badgeChoice != "yes" && badgeChoice != "no" {
		return errors.Newf("invalid badge choice: '%s'", badgeChoice)
	}

	u.Badge = badgeChoice

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.AirtableID,
			Fields: map[string]interface{}{
				fields.Badge: u.Badge,
			},
		}},
	}

	_, err := defaultTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "setting badge")
	}

	if defaultCache != nil {
		defaultCache.Delete(u.cacheKey())
	}

	return nil
}

func (u *User) SetFood(veg, gf, lact bool, comments string) error {
	u.Vegetarian = veg
	u.GlutenFree = gf
	u.LactoseIntolerant = lact
	u.FoodComments = comments

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.AirtableID,
			Fields: map[string]interface{}{
				fields.Vegetarian:        u.Vegetarian,
				fields.GlutenFree:        u.GlutenFree,
				fields.LactoseIntolerant: u.LactoseIntolerant,
				fields.FoodComments:      comments,
			},
		}},
	}

	_, err := defaultTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "setting food")
	}

	if defaultCache != nil {
		defaultCache.Delete(u.cacheKey())
	}

	return nil
}

func (u *User) GetCabinMates() ([]string, error) {
	if u.Cabin == "" {
		return nil, nil
	}

	var cabinMates []string
	response, err := query(fields.Cabin, u.Cabin, fields.TwitterName, fields.TwitterNameClean)
	if err != nil {
		return nil, err
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
	response, err := query(fields.TicketGroup, u.TicketGroup, fields.TwitterName, fields.CheckedIn)
	if err != nil {
		return nil, err
	}

	for _, c := range response.Records {
		ticketGroup = append(ticketGroup, TicketGroupEntry{
			TwitterName: toStr(c.Fields[fields.TwitterName]),
			CheckedIn:   c.Fields[fields.CheckedIn] == checked,
		})
	}

	return ticketGroup, nil
}

func (u *User) cacheKey() string { return u.TwitterNameClean }

func toStr(i interface{}) string {
	if i == nil {
		return ""
	}
	return i.(string)
}
