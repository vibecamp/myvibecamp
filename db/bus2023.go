package db

import (
	"github.com/cockroachdb/errors"
	"github.com/mehanizm/airtable"
	"github.com/vibecamp/myvibecamp/fields"
)

type BusSlot struct {
	Slot      string
	Purchased int
	Cap       int

	AirtableID string
}

func GetSlot(slot string) (*BusSlot, error) {
	var bs *BusSlot

	response, err := query(attendeesTable, fields.BusSlot, slot)

	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(response.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	rec := response.Records[0]
	bs = &BusSlot{
		Slot:      toStr(rec.Fields[fields.BusSlot]),
		Purchased: toInt(rec.Fields[fields.Purchased]),
		Cap:       toInt(rec.Fields[fields.Cap]),

		AirtableID: rec.ID,
	}

	return bs, err
}

func UpdateSlot(slot string, purchased int) error {
	bs, err := GetSlot(slot)

	if err != nil {
		return err
	}

	bs.Purchased = purchased

	if bs.Purchased > bs.Cap {
		return errors.New("purchased exceeds cap - please contact @orb_net")
	}

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: bs.AirtableID,
			Fields: map[string]interface{}{
				fields.Purchased: bs.Purchased,
			},
		}},
	}

	recvRecords, err := bus2023Table.UpdateRecordsPartial(r)

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
