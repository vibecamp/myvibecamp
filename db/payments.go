package db

import (
	"bytes"
	"encoding/gob"
	//"fmt"
	//"strings"
	//"time"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/lyoshenka/vibedata/fields"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
)

type Order struct {
	OrderID	      string
	UserName      string
	Total         int
	TotalTickets  int
	AdultCabin    int
	AdultTent     int
	AdultSat	  int
	ChildCabin    int
	ChildTent     int
	ChildSat	  int
	ToddlerCabin  int
	ToddlerTent   int
	ToddlerSat	  int
	Donation      int 
	StripeID      string
	PaymentStatus string

	AirtableID    string
}

func (order *Order) CreateOrder() error {
	if order.AirtableID != "" {
		err := errors.New("Order already exists")
		return err
	}

	recordsToSend := &airtable.Records{
		Records: []*airtable.Record{
			{
				Fields: map[string]interface{}{
					fields.UserName: order.UserName,
					fields.OrderID: order.OrderID,
					fields.Total: order.Total,
					fields.TotalTickets: order.TotalTickets,
					fields.AdultCabin: order.AdultCabin,
					fields.AdultTent: order.AdultTent,
					fields.AdultSat: order.AdultSat,
					fields.ChildCabin: order.ChildCabin,
					fields.ChildTent: order.ChildTent,
					fields.ChildSat: order.ChildSat,
					fields.ToddlerCabin: order.ToddlerCabin,
					fields.ToddlerTent: order.ToddlerTent,
					fields.ToddlerSat: order.ToddlerSat,
					fields.Donation: order.Donation,
					fields.PaymentID: order.StripeID,
					fields.PaymentStatus: order.PaymentStatus,
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

	order.AirtableID = recvRecords.Records[0].ID
	return nil
}

/*
*/

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
		AirtableID:			rec.ID,
		UserName:			toStr(rec.Fields[fields.UserName]),
		OrderID:			toStr(rec.Fields[fields.OrderID]),	
		Total:				toInt(rec.Fields[fields.Total]),
		TotalTickets:		toInt(rec.Fields[fields.TotalTickets]),
		AdultCabin:			toInt(rec.Fields[fields.AdultCabin]),
		AdultTent:			toInt(rec.Fields[fields.AdultTent]),
		AdultSat:			toInt(rec.Fields[fields.AdultSat]),
		ChildCabin:			toInt(rec.Fields[fields.ChildCabin]),
		ChildTent:			toInt(rec.Fields[fields.ChildTent]),
		ChildSat:			toInt(rec.Fields[fields.ChildSat]),
		ToddlerCabin:		toInt(rec.Fields[fields.ToddlerCabin]),
		ToddlerTent:		toInt(rec.Fields[fields.ToddlerTent]),
		ToddlerSat:			toInt(rec.Fields[fields.ToddlerSat]),
		Donation:			toInt(rec.Fields[fields.Donation]),
		StripeID:			toStr(rec.Fields[fields.PaymentID]),
		PaymentStatus:		toStr(rec.Fields[fields.PaymentStatus]),
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

func (order *Order) UpdateOrderStatus(paymentStatus string) error {
	order.PaymentStatus = paymentStatus

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: order.AirtableID,
			Fields: map[string]interface{}{
				fields.PaymentStatus:		order.PaymentStatus,
			},
		}},
	}

	_, err := ordersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating payment status")
	}

	if defaultCache != nil {
		defaultCache.Delete(order.cacheKey())
	}

	return nil
}

func toInt(i interface{}) int {
	if i == nil {
		return 0
	}
	num,_ := strconv.Atoi(i.(string))
	return num
}

func (o *Order) cacheKey() string { return o.OrderID }