package db

import (
	"bytes"
	"encoding/gob"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

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
	StripeID      string
	PaymentStatus string

	AirtableID string
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
					fields.Donation:      o.Donation,
					fields.PaymentID:     o.StripeID,
					fields.PaymentStatus: o.PaymentStatus,
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
		Donation:      toInt(rec.Fields[fields.Donation]),
		StripeID:      toStr(rec.Fields[fields.PaymentID]),
		PaymentStatus: toStr(rec.Fields[fields.PaymentStatus]),
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

func toInt(i interface{}) int {
	if i == nil {
		return 0
	}
	num, _ := strconv.Atoi(i.(string))
	return num
}

func (o *Order) cacheKey() string { return o.OrderID }
