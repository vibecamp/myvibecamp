package db

import (
	"github.com/mehanizm/airtable"
	"github.com/cockroachdb/errors"
	// log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

type Ticket struct {
	TicketID   string
	UserID     string
	OrderID    string
	ProductID  string
	Price      *Currency
	Upgrade    bool
	Enabled    bool
	CreatedAt  string

	AirtableID string
}

func buildTicketFromAirtableRecord(record *airtable.Record) *Ticket {
	ticket := &Ticket{
		AirtableID: record.ID,
		UserID:     toStr(record.Fields["User ID"]),
		OrderID:    toStr(record.Fields["Order ID"]),
		ProductID:  toStr(record.Fields["Product ID"]),
		Price:      CurrencyFromAirtableString(toStr(record.Fields["Price"])),
		Upgrade:    record.Fields["Upgrade"] == checked,
		Enabled:    record.Fields["Enabled"] == checked,
		CreatedAt:  toStr(record.Fields["Created At"]),
	}

	return ticket
}

func GetTicket(ticketID string) (*Ticket, error) {
	ticket, err := getTicketByID(ticketID)
	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("Ticket not found.")
		}

		return nil, err
	} else if ticket == nil {
		return nil, errors.New("Ticket not found.")
	}

	return ticket, nil
}

func getTicketByID(ticketID string) (*Ticket, error) {
	response, err := query(ticketsTable, "Ticket ID", ticketID)

	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(response.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	ticket := buildTicketFromAirtableRecord(response.Records[0])

	return ticket, nil
}

func GetTicketsForOrder(orderID string) ([]*Ticket, error) {
	response, err := query(ticketsTable, fields.OrderID, orderID)

	if err != nil {
		return []*Ticket{}, err
	}

	if response == nil {
		return []*Ticket{}, errors.Wrap(ErrNoRecords, "")
	}

	tickets := []*Ticket{}

	for _, record := range response.Records {
		tickets = append(tickets, buildTicketFromAirtableRecord(record))
	}

	return tickets, nil
}
