package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

type Item struct {
	Id       string `json:"id"`
	Quantity int    `json:"quantity"`
	Amount   int    `json:"amount"`
}

type Order struct {
	OrderID       string
	UserName      string
	Total         *Currency
	ProcessingFee *Currency
	TotalTickets  int
	AdultCabin    int
	AdultTent     int
	AdultSat      int
	ChildCabin    int
	ChildTent     int
	ChildSat      int
	ToddlerCabin  int
	ToddlerTent   int
	ToddlerSat    int
	Donation      int
	CardPacks     int
	StripeID      string
	PaymentStatus string
	Date          string

	AirtableID string
}

var CartFields = map[string]string{
	fields.AdultCabin:   "adult-cabin",
	fields.AdultTent:    "adult-tent",
	fields.AdultSat:     "adult-sat",
	fields.ChildCabin:   "child-cabin",
	fields.ChildTent:    "child-tent",
	fields.ChildSat:     "child-sat",
	fields.ToddlerCabin: "toddler-cabin",
	fields.ToddlerTent:  "toddler-tent",
	fields.ToddlerSat:   "toddler-sat",
	fields.Donation:     "donation",
}

func (o *Order) CreateOrder() error {
	if o.AirtableID != "" {
		err := errors.New("Order already exists")
		return err
	}

	recordsToSend := &airtable.Records{
		Records: []*airtable.Record{
			{
				Fields: map[string]interface{}{
					fields.UserName:      o.UserName,
					fields.OrderID:       o.OrderID,
					fields.Total:         o.Total.ToFloat(),
					fields.ProcessingFee: o.ProcessingFee.ToFloat(),
					fields.TotalTickets:  o.TotalTickets,
					fields.AdultCabin:    o.AdultCabin,
					fields.AdultTent:     o.AdultTent,
					fields.AdultSat:      o.AdultSat,
					fields.ChildCabin:    o.ChildCabin,
					fields.ChildTent:     o.ChildTent,
					fields.ChildSat:      o.ChildSat,
					fields.ToddlerCabin:  o.ToddlerCabin,
					fields.ToddlerTent:   o.ToddlerTent,
					fields.ToddlerSat:    o.ToddlerSat,
					fields.CardPacks:     o.CardPacks,
					fields.Donation:      o.Donation,
					fields.PaymentID:     o.StripeID,
					fields.PaymentStatus: o.PaymentStatus,
					fields.Date:          o.Date,
				},
			},
		},
	}

	recvRecords, err := ordersTable.AddRecords(recordsToSend)
	if err != nil {
		return errors.Wrap(err, "Error creating your tickets - contact orb_net")
	}

	if recvRecords == nil || len(recvRecords.Records) == 0 {
		return errors.Wrap(ErrNoRecords, "")
	} else if len(recvRecords.Records) != 1 {
		return errors.Wrap(ErrManyRecords, "")
	}

	o.AirtableID = recvRecords.Records[0].ID
	return nil
}

func GetOrder(orderId string) (*Order, error) {
	if defaultCache != nil {
		if o, found := defaultCache.Get(orderId); found {
			log.Trace("order cache hit")
			var order Order
			err := gob.NewDecoder(bytes.NewBuffer(o.([]byte))).Decode(&order)
			if err != nil {
				return nil, errors.Wrap(err, "cache hit")
			}
			return &order, nil
		}
	}

	order, err := getOrderByField(fields.OrderID, orderId)
	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("No order found! There may have been a mistake")
		} else if errors.Is(err, ErrManyRecords) {
			err = errors.New("Multiple orders found, looks like we may have had an issue")
		}
		return nil, err
	} else if order == nil {
		return nil, errors.New("no order found, but no error from db 🤔")
	}

	return order, nil
}

func GetOrderByPaymentID(paymentId string) (*Order, error) {
	order, err := getOrderByField(fields.PaymentID, paymentId)
	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("No order found! There may have been a mistake")
		} else if errors.Is(err, ErrManyRecords) {
			err = errors.New("Multiple orders found, looks like we may have had an issue")
		}
		return nil, err
	} else if order == nil {
		return nil, errors.New("no order found, but no error from db 🤔")
	}

	return order, nil
}

func getOrderByField(field, value string) (*Order, error) {
	response, err := query(ordersTable, field, value) // get all fields
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(response.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	rec := response.Records[0]

	o := &Order{
		AirtableID:    rec.ID,
		UserName:      toStr(rec.Fields[fields.UserName]),
		OrderID:       toStr(rec.Fields[fields.OrderID]),
		Total:         CurrencyFromAirtableString(toStr(rec.Fields[fields.Total])),
		ProcessingFee: CurrencyFromAirtableString(toStr(rec.Fields[fields.ProcessingFee])),
		Donation:      CurrencyFromAirtableString(toStr(rec.Fields[fields.Donation])).Dollars,
		TotalTickets:  toInt(rec.Fields[fields.TotalTickets]),
		AdultCabin:    toInt(rec.Fields[fields.AdultCabin]),
		AdultTent:     toInt(rec.Fields[fields.AdultTent]),
		AdultSat:      toInt(rec.Fields[fields.AdultSat]),
		ChildCabin:    toInt(rec.Fields[fields.ChildCabin]),
		ChildTent:     toInt(rec.Fields[fields.ChildTent]),
		ChildSat:      toInt(rec.Fields[fields.ChildSat]),
		ToddlerCabin:  toInt(rec.Fields[fields.ToddlerCabin]),
		ToddlerTent:   toInt(rec.Fields[fields.ToddlerTent]),
		ToddlerSat:    toInt(rec.Fields[fields.ToddlerSat]),
		StripeID:      toStr(rec.Fields[fields.PaymentID]),
		PaymentStatus: toStr(rec.Fields[fields.PaymentStatus]),
		Date:          toStr(rec.Fields[fields.Date]),
	}

	if defaultCache != nil {
		var b bytes.Buffer
		err := gob.NewEncoder(&b).Encode(*o)
		if err != nil {
			return nil, errors.Wrap(err, "cache save")
		}
		defaultCache.Set(o.cacheKey(), b.Bytes(), 0)
	}

	return o, nil
}

func (o *Order) UpdateOrderStatus(paymentStatus string) error {
	o.PaymentStatus = paymentStatus

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: o.AirtableID,
			Fields: map[string]interface{}{
				fields.PaymentStatus: o.PaymentStatus,
			},
		}},
	}

	_, err := ordersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating payment status")
	}

	if defaultCache != nil {
		defaultCache.Delete(o.cacheKey())
	}

	return nil
}

func (o *Order) ReplaceCart(a *Order) error {
	a.AirtableID = o.AirtableID
	a.StripeID = o.StripeID
	a.OrderID = o.OrderID
	a.UserName = o.UserName
	r := &airtable.Records{
		Records: []*airtable.Record{
			{
				ID: a.AirtableID,
				Fields: map[string]interface{}{
					fields.Total:         a.Total.ToFloat(),
					fields.ProcessingFee: a.ProcessingFee.ToFloat(),
					fields.TotalTickets:  a.TotalTickets,
					fields.AdultCabin:    a.AdultCabin,
					fields.AdultTent:     a.AdultTent,
					fields.AdultSat:      a.AdultSat,
					fields.ChildCabin:    a.ChildCabin,
					fields.ChildTent:     a.ChildTent,
					fields.ChildSat:      a.ChildSat,
					fields.ToddlerCabin:  a.ToddlerCabin,
					fields.ToddlerTent:   a.ToddlerTent,
					fields.ToddlerSat:    a.ToddlerSat,
					fields.Donation:      a.Donation,
				},
			},
		},
	}

	_, err := ordersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "Error updating order info - contact @orb_net if this persists")
	}

	if defaultCache != nil {
		defaultCache.Delete(o.cacheKey())
	}
	return nil
}

func (o *Order) IsEqual(a *Order) bool {
	if o.Total.ToString() != a.Total.ToString() {
		return false
	}

	oCart := o.toCart()
	aCart := a.toCart()

	for item := range CartFields {
		if oCart[item] != aCart[item] {
			return false
		}
	}

	return true
}

func (o *Order) toCart() map[string]int {
	return map[string]int{
		"adult-cabin":   o.AdultCabin,
		"adult-tent":    o.AdultTent,
		"adult-sat":     o.AdultSat,
		"child-cabin":   o.ChildCabin,
		"child-tent":    o.ChildTent,
		"child-sat":     o.ChildSat,
		"toddler-cabin": o.ToddlerCabin,
		"toddler-tent":  o.ToddlerTent,
		"toddler-sat":   o.ToddlerSat,
		"donation":      o.Donation,
	}
}

func toInt(i interface{}) int {
	if i == nil {
		return 0
	}
	num, _ := strconv.Atoi(i.(string))
	return num
}

func (o *Order) cacheKey() string { return o.OrderID }

type ItemType int64

const (
	Undefined ItemType = iota
	AdultCabin
	AdultTent
	AdultSat
	ChildCabin
	ChildTent
	ChildSat
	ToddlerCabin
	ToddlerTent
	ToddlerSat
	Donation
	SatToTentUpgrade
	TentToCabinUpgrade
)

func (i ItemType) String() string {
	switch i {
	case AdultCabin:
		return "adult-cabin"
	case AdultTent:
		return "adult-tent"
	case AdultSat:
		return "adult-sat"
	case ChildCabin:
		return "child-cabin"
	case ChildTent:
		return "child-tent"
	case ChildSat:
		return "child-sat"
	case ToddlerCabin:
		return "toddler-cabin"
	case ToddlerTent:
		return "toddler-tent"
	case ToddlerSat:
		return "toddler-sat"
	case Donation:
		return "donation"
	case SatToTentUpgrade:
		return "sat-tent-upgrade"
	case TentToCabinUpgrade:
		return "tent-cabin-upgrade"
	}

	return "unknown"
}

func MakeStringCart(i []Item) string {
	cart := ""
	for idx, item := range i {
		if idx != 0 {
			cart += ","
		}

		if item.Id == Donation.String() && item.Amount > 0 && item.Quantity > 0 {
			cart += fmt.Sprintf("%s %d", item.Id, item.Amount)
		} else if item.Quantity > 0 {
			cart += fmt.Sprintf("%s %d", item.Id, item.Quantity)
		}
	}
	return cart
}

func StringCartToItem(cart string) []Item {
	cartItems := strings.Split(cart, ",")
	items := []Item{}

	for _, item := range cartItems {
		splitItem := strings.Split(item, " ")

		if len(splitItem) == 1 {
			break
		}

		numVal, err := strconv.Atoi(splitItem[1])

		if err != nil {
			log.Error("Atoi error: %v", err)
		}

		newItem := Item{
			Id:       splitItem[0],
			Quantity: 0,
			Amount:   0,
		}

		if newItem.Id == Donation.String() {
			newItem.Amount = numVal
		} else {
			newItem.Quantity = numVal
		}

		items = append(items, newItem)
	}

	return items
}
