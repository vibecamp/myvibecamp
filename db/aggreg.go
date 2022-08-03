package db

import (
	"bytes"
	"encoding/gob"
	"strconv"
	"strings"

	//"fmt"
	//"strings"
	//"time"

	"github.com/cockroachdb/errors"
	"github.com/lyoshenka/vibedata/fields"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
)

type Constant struct {
	Name  string
	Value int

	AirtableID string
}

type Aggregation struct {
	Name     string
	Quantity int
	Revenue  int

	AirtableID string
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
		AirtableID: rec.ID,
		Name:       toStr(rec.Fields[fields.Name]),
		Value:      toInt(rec.Fields[fields.Value]),
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
				fields.Value: value,
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
	revenueStr := toStr(rec.Fields[fields.Revenue])[1:]
	log.Debugf("%s", revenueStr)
	log.Debugf("%s", revenueStr[:len(revenueStr)-3])
	currencyInts, _ := strconv.Atoi(revenueStr[:len(revenueStr)-3])
	currencyCents, _ := strconv.Atoi(revenueStr[len(revenueStr)-2:])
	revenue := currencyInts*100 + currencyCents
	a := &Aggregation{
		AirtableID: rec.ID,
		Name:       toStr(rec.Fields[fields.Name]),
		Quantity:   toInt(rec.Fields[fields.Quantity]),
		Revenue:    revenue,
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

func GetAggregations() ([]*Aggregation, error) {
	response, err := aggregationsTable.GetRecords().
		InStringFormat("US/Eastern", "en").
		Do()
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	}

	var aggregations []*Aggregation
	for _, rec := range response.Records {
		revenueStr := strings.Replace(toStr(rec.Fields[fields.Revenue])[1:], ",", "", -1)
		currencyInts, _ := strconv.Atoi(revenueStr[:len(revenueStr)-3])
		currencyCents, _ := strconv.Atoi(revenueStr[len(revenueStr)-2:])
		revenue := currencyInts*100 + currencyCents

		aggregations = append(aggregations, &Aggregation{
			AirtableID: rec.ID,
			Name:       toStr(rec.Fields[fields.Name]),
			Quantity:   toInt(rec.Fields[fields.Quantity]),
			Revenue:    revenue,
		})
	}

	return aggregations, nil
}

func (agg *Aggregation) UpdateAggregation(quantity int, revenue int) error {
	agg.Quantity = quantity
	agg.Revenue = revenue

	revenueStr := "$" + strconv.Itoa(revenue/100) + "." + strconv.Itoa(revenue%100)

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: agg.AirtableID,
			Fields: map[string]interface{}{
				fields.Quantity: agg.Quantity,
				fields.Revenue:  revenueStr,
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
	} else {
		return errors.New("No such aggregation")
	}

	revenueStr := "$" + strconv.Itoa(agg.Revenue/100) + "." + strconv.Itoa(agg.Revenue%100)

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: agg.AirtableID,
			Fields: map[string]interface{}{
				fields.Quantity: agg.Quantity,
				fields.Revenue:  revenueStr,
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

func (agg *Aggregation) MakeUpdatedRecord(order *Order) *airtable.Record {
	if agg.Name == fields.TotalTicketsSold || agg.Name == fields.SoftLaunchSold {
		agg.Quantity += order.TotalTickets
		agg.Revenue += (order.Total - order.Donation) * 100
	} else if agg.Name == fields.CabinSold {
		agg.Quantity += order.AdultCabin + order.ChildCabin + order.ToddlerCabin
		agg.Revenue += ((order.AdultCabin * 590) + (order.ChildCabin * 380)) * 100
	} else if agg.Name == fields.TentSold {
		agg.Quantity += order.AdultTent + order.ChildTent + order.ToddlerTent
		agg.Revenue += ((order.AdultTent * 420) + (order.ChildTent * 210)) * 100
	} else if agg.Name == fields.SatSold {
		agg.Quantity += order.AdultSat + order.ChildSat + order.ToddlerSat
		agg.Revenue += ((order.AdultSat * 140) + (order.ChildSat * 70)) * 100
	} else if agg.Name == fields.AdultSold {
		agg.Quantity += order.AdultCabin + order.AdultTent + order.AdultSat
		agg.Revenue += ((order.AdultCabin * 590) + (order.AdultTent * 420) + (order.AdultSat * 140)) * 100
	} else if agg.Name == fields.ChildSold {
		agg.Quantity += order.ChildCabin + order.ChildTent + order.ChildSat
		agg.Revenue += ((order.ChildCabin * 380) + (order.ChildTent * 210) + (order.ChildSat * 70)) * 100
	} else if agg.Name == fields.DonationsRecv {
		if order.Donation > 0 {
			agg.Quantity += 1
			agg.Revenue += order.Donation * 100
		}
	} else if agg.Name == fields.FullSold {
		agg.Quantity += order.AdultCabin + order.AdultTent + order.ChildCabin + order.ChildTent + order.ToddlerCabin + order.ToddlerTent
		agg.Revenue += ((order.AdultCabin * 590) + (order.AdultTent * 420) + (order.ChildCabin * 380) + (order.ChildTent * 210)) * 100
	}

	cents := agg.Revenue % 100
	centsStr := "00"
	if cents < 10 {
		centsStr = "0" + strconv.Itoa(cents)
	} else {
		centsStr = strconv.Itoa(cents)
	}
	// "$" +
	revenueStr := strconv.Itoa(agg.Revenue/100) + "." + centsStr
	revenueFloat, err := strconv.ParseFloat(revenueStr, 64)
	if err != nil {
		log.Errorf("Error converting revenue %s %v", revenueStr, err)
	}

	r := &airtable.Record{
		ID: agg.AirtableID,
		Fields: map[string]interface{}{
			fields.Quantity: agg.Quantity,
			fields.Revenue:  revenueFloat,
		},
	}

	return r
}

func UpdateAggregations(order *Order) error {
	aggregations, err := GetAggregations()
	if err != nil {
		return err
	}

	var records []*airtable.Record

	for _, element := range aggregations {
		if order.Donation > 0 && element.Name == fields.DonationsRecv {
			records = append(records, element.MakeUpdatedRecord(order))
		} else if order.TotalTickets > 0 {
			records = append(records, element.MakeUpdatedRecord(order))
		}
	}

	r := &airtable.Records{
		Records: records,
	}

	_, err = aggregationsTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating aggregations")
	}

	return nil
}

func (c *Constant) cacheKey() string    { return "cons-" + c.Name }
func (a *Aggregation) cacheKey() string { return "agg-" + a.Name }
