package db

import (
	"bytes"
	"encoding/gob"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

type User struct {
	UserID            string
	UserName          string
	TwitterName       string
	Name              string
	Email             string
	AdmissionLevel    string
	TicketType        string
	Barcode           string
	OrderNotes        string
	OrderID           string
	CheckedIn         bool
	Badge             bool
	Vegetarian        bool
	GlutenFree        bool
	LactoseIntolerant bool
	FoodComments      string
	TicketID          string
	DiscordName       string
	TicketPath        string

	AirtableID string
}

type SponsorshipUser struct {
	UserName    string
	Name        string
	TwitterName string
	Email       string
	TicketLimit int
	Discount    *Currency

	AirtableID string
}

func init() {
	gob.Register(User{})
	gob.Register(SoftLaunchUser{})
	gob.Register(Order{})
}

func (u *User) UpdateUser() error {
	if u.AirtableID == "" {
		err := errors.New("No airtable ID")
		return err
	}

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.AirtableID,
			Fields: map[string]interface{}{
				fields.UserName:          u.UserName,
				fields.TwitterName:       u.TwitterName,
				fields.Name:              u.Name,
				fields.Email:             u.Email,
				fields.AdmissionLevel:    u.AdmissionLevel,
				fields.TicketType:        u.TicketType,
				fields.Barcode:           u.Barcode,
				fields.OrderNotes:        u.OrderNotes,
				fields.OrderID:           u.OrderID,
				fields.CheckedIn:         u.CheckedIn,
				fields.Badge:             u.Badge,
				fields.Vegetarian:        u.Vegetarian,
				fields.GlutenFree:        u.GlutenFree,
				fields.LactoseIntolerant: u.LactoseIntolerant,
				fields.FoodComments:      u.FoodComments,
				fields.TicketID:          "",
				fields.DiscordName:       u.DiscordName,
				fields.TicketPath:        u.TicketPath,
			},
		}},
	}

	recvRecords, err := usersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating attendee record")
	}

	if recvRecords == nil || len(recvRecords.Records) == 0 {
		return errors.Wrap(ErrNoRecords, "")
	} else if len(recvRecords.Records) != 1 {
		return errors.Wrap(ErrManyRecords, "")
	}

	return nil
}

func (u *User) CreateUser() error {
	if u.AirtableID != "" {
		err := errors.New("User already exists")
		return err
	}

	r := &airtable.Records{
		Records: []*airtable.Record{
			{
				Fields: map[string]interface{}{
					fields.UserName:          u.UserName,
					fields.TwitterName:       u.TwitterName,
					fields.Name:              u.Name,
					fields.Email:             u.Email,
					fields.AdmissionLevel:    u.AdmissionLevel,
					fields.TicketType:        u.TicketType,
					fields.Barcode:           u.Barcode,
					fields.OrderNotes:        u.OrderNotes,
					fields.OrderID:           u.OrderID,
					fields.CheckedIn:         u.CheckedIn,
					fields.Badge:             u.Badge,
					fields.Vegetarian:        u.Vegetarian,
					fields.GlutenFree:        u.GlutenFree,
					fields.LactoseIntolerant: u.LactoseIntolerant,
					fields.FoodComments:      u.FoodComments,
					fields.TicketID:          "",
					fields.DiscordName:       u.DiscordName,
					fields.TicketPath:        u.TicketPath,
				},
			},
		},
	}

	recvRecords, err := usersTable.AddRecords(r)
	if err != nil {
		return errors.Wrap(err, "creating attendee record")
	}

	if recvRecords == nil || len(recvRecords.Records) == 0 {
		return errors.Wrap(ErrNoRecords, "")
	} else if len(recvRecords.Records) != 1 {
		return errors.Wrap(ErrManyRecords, "")
	}

	u.AirtableID = recvRecords.Records[0].ID
	return nil
}

func GetUserFromBarcode(barcode string) (*User, error) {
	return getUserByField(fields.Barcode, barcode)
}

func GetUser(userName string) (*User, error) {
	cleanName := strings.ToLower(userName)
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

	user, err := getUserByField(fields.UserName, cleanName)
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

func getUserByField(field, value string) (*User, error) {
	response, err := query(usersTable, field, value) // get all fields
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(response.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	rec := response.Records[0]

	u := &User{
		AirtableID:        rec.ID,
		UserID:            toStr(rec.Fields[fields.UserID]),
		UserName:          toStr(rec.Fields[fields.UserName]),
		TwitterName:       toStr(rec.Fields[fields.TwitterName]),
		Name:              toStr(rec.Fields[fields.Name]),
		Email:             toStr(rec.Fields[fields.Email]),
		TicketType:        toStr(rec.Fields[fields.TicketType]),
		AdmissionLevel:    toStr(rec.Fields[fields.AdmissionLevel]),
		CheckedIn:         rec.Fields[fields.CheckedIn] == checked,
		Barcode:           toStr(rec.Fields[fields.Barcode]),
		OrderNotes:        toStr(rec.Fields[fields.OrderNotes]),
		Badge:             rec.Fields[fields.Badge] == checked,
		Vegetarian:        rec.Fields[fields.Vegetarian] == checked,
		GlutenFree:        rec.Fields[fields.GlutenFree] == checked,
		LactoseIntolerant: rec.Fields[fields.LactoseIntolerant] == checked,
		FoodComments:      toStr(rec.Fields[fields.FoodComments]),
		TicketPath:        toStr(rec.Fields[fields.TicketPath]),
		DiscordName:       toStr(rec.Fields[fields.DiscordName]),
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

func GetSponsorshipUser(userName string) (*SponsorshipUser, error) {
	cleanName := strings.ToLower(userName)
	if defaultCache != nil {
		if u, found := defaultCache.Get("sp-" + cleanName); found {
			log.Trace("user cache hit")
			var user SponsorshipUser
			err := gob.NewDecoder(bytes.NewBuffer(u.([]byte))).Decode(&user)
			if err != nil {
				return nil, errors.Wrap(err, "cache hit")
			}
			return &user, nil
		}
	}

	user, err := getSponsorshipUserByField(fields.UserName, cleanName)
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

func getSponsorshipUserByField(field, value string) (*SponsorshipUser, error) {
	response, err := query(sponsorshipTable, field, value) // get all fields
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

	u := &SponsorshipUser{
		AirtableID:  rec.ID,
		UserName:    toStr(rec.Fields[fields.UserName]),
		TwitterName: toStr(rec.Fields[fields.TwitterName]),
		Name:        toStr(rec.Fields[fields.Name]),
		Email:       toStr(rec.Fields[fields.Email]),
		Discount:    CurrencyFromAirtableString(toStr(rec.Fields[fields.Discount])),
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

func (u *User) SetBadge(badgeChoice string) error {
	if badgeChoice != "yes" && badgeChoice != "no" {
		return errors.Newf("invalid badge choice: '%s'", badgeChoice)
	}

	u.Badge = badgeChoice == "yes"

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.AirtableID,
			Fields: map[string]interface{}{
				fields.Badge: u.Badge,
			},
		}},
	}

	_, err := usersTable.UpdateRecordsPartial(r)
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

	_, err := usersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "setting food")
	}

	if defaultCache != nil {
		defaultCache.Delete(u.cacheKey())
	}

	return nil
}

func (u *User) Set2023Logistics(badge, veg, gf, lact bool, comments string, discordName string) error {
	u.Badge = badge
	u.Vegetarian = veg
	u.GlutenFree = gf
	u.LactoseIntolerant = lact
	u.FoodComments = comments
	u.DiscordName = discordName

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.AirtableID,
			Fields: map[string]interface{}{
				fields.Vegetarian:        u.Vegetarian,
				fields.GlutenFree:        u.GlutenFree,
				fields.LactoseIntolerant: u.LactoseIntolerant,
				fields.FoodComments:      comments,
				fields.Badge:             u.Badge,
				fields.DiscordName:       u.DiscordName,
			},
		}},
	}

	_, err := usersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "setting food")
	}

	if defaultCache != nil {
		defaultCache.Delete(u.cacheKey())
	}

	return nil
}

func (u *User) UpdateOrderID(orderId string) error {
	u.OrderID = orderId

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.AirtableID,
			Fields: map[string]interface{}{
				fields.OrderID: u.OrderID,
			},
		}},
	}

	_, err := usersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "setting order id")
	}

	if defaultCache != nil {
		defaultCache.Delete(u.cacheKey())
	}

	return nil
}

func (u *User) UpdateTicketId(ticketId string) error {
	u.TicketID = ticketId

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.AirtableID,
			Fields: map[string]interface{}{
				fields.TicketID: u.TicketID,
			},
		}},
	}

	_, err := usersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "setting ticket id")
	}

	if defaultCache != nil {
		defaultCache.Delete(u.cacheKey())
	}

	return nil
}

func (u *User) SetCheckedIn() error {
	u.CheckedIn = true

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: u.AirtableID,
			Fields: map[string]interface{}{
				fields.CheckedIn: u.CheckedIn,
			},
		}},
	}

	_, err := usersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "checking in "+u.UserName)
	}

	if defaultCache != nil {
		defaultCache.Delete(u.cacheKey())
	}

	return nil
}

func GetUserByDiscord(discordName string) (*User, error) {
	user, err := getUserByField(fields.DiscordName, discordName)
	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("user not found")
		} else if errors.Is(err, ErrManyRecords) {
			err = errors.New("too many users")
		}
		return nil, err
	} else if user == nil {
		return nil, errors.New("no user found, but no error from db 🤔")
	}

	return user, nil
}

/*
func (u *User) GetCabinMates() ([]string, error) {
	if u.Cabin == "" {
		return nil, nil
	}

	var cabinMates []string
	response, err := query(defaultTable, fields.Cabin, u.Cabin, fields.TwitterName, fields.TwitterNameClean)
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
*/

func (u *User) GetTicketGroup() ([]*User, error) {
	if u.OrderID == "" {
		return []*User{u}, nil
	}

	response, err := query(usersTable, fields.OrderID, u.OrderID, fields.UserName)
	if err != nil {
		return nil, err
	}

	group := make([]*User, len(response.Records))
	for i := 0; i < len(group); i++ {
		group[i], err = GetUser(toStr(response.Records[i].Fields[fields.UserName]))
		if err != nil {
			return nil, err
		}
	}

	return group, nil
}

func (u *User) cacheKey() string            { return u.UserName }
func (u *SoftLaunchUser) cacheKey() string  { return "sl-" + u.UserName }
func (u *ChaosModeUser) cacheKey() string   { return "cm-" + u.UserName }
func (u *SponsorshipUser) cacheKey() string { return "sp-" + u.UserName }

func (u *User) HasCheckinPermission() bool {
	if u.AdmissionLevel == "Staff" {
		return true
	}

	helping := []string{
		"konstell2", "thermestor", "dancinghorse16",
	}

	for _, h := range helping {
		if h == u.UserName {
			return true
		}
	}

	return false
}

func toStr(i interface{}) string {
	if i == nil {
		return ""
	}
	return i.(string)
}

// CacheWarmup fetches every user to warm up the cache
func CacheWarmup() {
	offset := ""

	for {
		response, err := usersTable.GetRecords().
			WithOffset(offset).
			ReturnFields(fields.UserName).
			InStringFormat("US/Eastern", "en").
			Do()

		if err != nil {
			log.Errorf("%+v", err)
			return
		}

		for _, r := range response.Records {
			GetUser(toStr(r.Fields[fields.UserName]))
			time.Sleep(5 * time.Second)
		}

		if response.Offset == "" {
			break
		}
		offset = response.Offset
	}
}
