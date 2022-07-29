package db

import (
	//"bytes"
	//"encoding/gob"
	//"fmt"
	//"strings"
	//"time"

	"github.com/cockroachdb/errors"
	"github.com/lyoshenka/vibedata/fields"
	"github.com/mehanizm/airtable"
	//log "github.com/sirupsen/logrus"
)

type Order struct {
	OrderID	      string
	UserName      string
	Total         int
	TotalTickets  int
	AdultCabin    int
	AdultTent     int
	ChildCabin    int
	ChildTent     int
	ToddlerCabin  int
	ToddlerTent   int
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
					fields.ChildCabin: order.ChildCabin,
					fields.ChildTent: order.ChildTent,
					fields.ToddlerCabin: order.ToddlerCabin,
					fields.ToddlerTent: order.ToddlerTent,
					fields.Donation: order.Donation,
					fields.PaymentID: order.StripeID,
					fields.PaymentStatus: order.PaymentStatus,
				},
			},
		},
	}

	recvRecords, err := ordersTable.AddRecords(recordsToSend)
	if err != nil {
		err = errors.New("Error creating your tickets - contact orb_net")
		return err
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
func GetOrder(orderId string) (order *Order, error) {}
func (order *Order) UpdateOrder() error {}
*/