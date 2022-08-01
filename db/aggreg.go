package db

import (
	"bytes"
	"encoding/gob"
	//"fmt"
	//"strings"
	//"time"

	"github.com/cockroachdb/errors"
	"github.com/lyoshenka/vibedata/fields"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
)

type Constant struct {
	Name		string
	Value		int

	AirtableID	string
}

type Aggregation struct {
	Name		string
	Quantity	int
	Revenue		int

	AirtableID	string
}

func GetConstant(constantName string) (*Constant, error) {
	if defaultCache != nil {
		if c, found := defaultCache.Get("cons-" + constantName); found {
			log.Trace("order cache hit")
			var dbConst Constant
			err := gob.NewDecoder(bytes.NewBuffer(c.([]byte))).Decode(&dbConst)
			if err != nil {
				return nil, errors.Wrap(err, "cache hit")
			}
			return &dbConst, nil
		}
	}

	dbConst, err := getConstantByField(fields.Name, constantName)
	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("No constant found! There may have been a mistake")
		} else if errors.Is(err, ErrManyRecords) {
			err = errors.New("Multiple constants found, looks like we may have had an issue")
		}
		return nil, err
	} else if dbConst == nil {
		return nil, errors.New("no constant found, but no error from db 🤔")
	}

	return dbConst, nil
}

func getConstantByField(field, value string) (*Constant, error) {
	response, err := query(constantsTable, field, value) // get all fields
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(response.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	rec := response.Records[0]

	c := &Constant{
		AirtableID:		rec.ID,
		Name:			toStr(rec.Fields[fields.Name]),
		Value:			toInt(rec.Fields[fields.Value]),
	}

	if defaultCache != nil {
		var b bytes.Buffer
		err := gob.NewEncoder(&b).Encode(*c)
		if err != nil {
			return nil, errors.Wrap(err, "cache save")
		}
		defaultCache.Set(c.cacheKey(), b.Bytes(), 0)
	}

	return c, nil
}

func (dbConst *Constant) UpdateConstantValue(value int) error {
	dbConst.Value = value

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: dbConst.AirtableID,
			Fields: map[string]interface{}{
				fields.Value:		value,
			},
		}},
	}

	_, err := constantsTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating constant value")
	}

	if defaultCache != nil {
		defaultCache.Delete(dbConst.cacheKey())
	}

	return nil
}

func GetAggregation(aggName string) (*Aggregation, error) {
	if defaultCache != nil {
		if a, found := defaultCache.Get("agg-" + aggName); found {
			log.Trace("order cache hit")
			var agg Aggregation
			err := gob.NewDecoder(bytes.NewBuffer(a.([]byte))).Decode(&agg)
			if err != nil {
				return nil, errors.Wrap(err, "cache hit")
			}
			return &agg, nil
		}
	}

	agg, err := getAggregationByField(fields.Name, aggName)
	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("No aggregation found! There may have been a mistake")
		} else if errors.Is(err, ErrManyRecords) {
			err = errors.New("Multiple aggregations found, looks like we may have had an issue")
		}
		return nil, err
	} else if agg == nil {
		return nil, errors.New("no aggregation found, but no error from db 🤔")
	}

	return agg, nil
}

func getAggregationByField(field, value string) (*Aggregation, error) {
	response, err := query(aggregationsTable, field, value) // get all fields
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	} else if len(response.Records) != 1 {
		return nil, errors.Wrap(ErrManyRecords, "")
	}

	rec := response.Records[0]

	log.Debugf("%v", rec.Fields[fields.Revenue])
	a := &Aggregation{
		AirtableID:		rec.ID,
		Name:			toStr(rec.Fields[fields.Name]),
		Quantity:		toInt(rec.Fields[fields.Value]),
		Revenue:		0,
		//toInt(rec.Fields[fields.Revenue]),
	}

	if defaultCache != nil {
		var b bytes.Buffer
		err := gob.NewEncoder(&b).Encode(*a)
		if err != nil {
			return nil, errors.Wrap(err, "cache save")
		}
		defaultCache.Set(a.cacheKey(), b.Bytes(), 0)
	}

	return a, nil
}

func (agg *Aggregation) UpdateAggregation(quantity int, revenue int) error {
	agg.Quantity = quantity
	agg.Revenue = revenue

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: agg.AirtableID,
			Fields: map[string]interface{}{
				fields.Quantity:	agg.Quantity,
				fields.Revenue:		agg.Revenue,
			},
		}},
	}

	_, err := aggregationsTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating aggregation")
	}

	if defaultCache != nil {
		defaultCache.Delete(agg.cacheKey())
	}

	return nil
}

func (agg *Aggregation) UpdateAggregationFromOrder(order *Order) error {
	if agg.Name == fields.TotalTicketsSold || agg.Name == fields.SoftLaunchSold {
		agg.Quantity += order.TotalTickets
		agg.Revenue += order.Total - order.Donation
	} else if agg.Name == fields.CabinSold {
		agg.Quantity += order.AdultCabin + order.ChildCabin + order.ToddlerCabin
		agg.Revenue += (order.AdultCabin * 590) + (order.ChildCabin * 380)
	} else if agg.Name == fields.TentSold {
		agg.Quantity += order.AdultTent + order.ChildTent + order.ToddlerTent
		agg.Revenue += (order.AdultTent * 420) + (order.ChildTent * 210)
	} else if agg.Name == fields.SatSold {
		agg.Quantity += order.AdultSat + order.ChildSat + order.ToddlerSat
		agg.Revenue += (order.AdultSat * 140) + (order.ChildSat * 70)
	} else if agg.Name == fields.AdultSold {
		agg.Quantity += order.AdultCabin + order.AdultTent + order.AdultSat
		agg.Revenue += (order.AdultCabin * 590) + (order.AdultTent * 420) + (order.AdultSat * 140)
	} else if agg.Name == fields.ChildSold {
		agg.Quantity += order.ChildCabin + order.ChildTent + order.ChildSat
		agg.Revenue += (order.ChildCabin * 380) + (order.ChildTent * 210) + (order.ChildSat * 70)
	} else if agg.Name == fields.DonationsRecv {
		if order.Donation > 0 {
			agg.Quantity += 1
			agg.Revenue += order.Donation
		}
	} else if agg.Name == fields.ScholarshipTickets {
		if order.Donation > 0 {
			agg.Revenue += order.Donation
			agg.Quantity = (int)(agg.Revenue / 420)
		}
	} else {
		return errors.New("No such aggregation")
	}

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: agg.AirtableID,
			Fields: map[string]interface{}{
				fields.Quantity:	agg.Quantity,
				fields.Revenue:		agg.Revenue,
			},
		}},
	}

	_, err := aggregationsTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating aggregation")
	}

	if defaultCache != nil {
		defaultCache.Delete(agg.cacheKey())
	}

	return nil
}

func (c *Constant) cacheKey() string { return "cons-" + c.Name }
func (a *Aggregation) cacheKey() string { return "agg-" + a.Name }