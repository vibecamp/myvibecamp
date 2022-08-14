package db

import (
	"bytes"
	"encoding/gob"
	"math"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

var stripeFee float64 = 0.03

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

type Currency struct {
	Dollars int
	Cents   int
}

func CurrencyFromAirtableString(str string) *Currency {
	revenueStr := strings.Replace(str[1:], ",", "", -1)
	currencyInts, _ := strconv.Atoi(revenueStr[:len(revenueStr)-3])
	currencyCents, _ := strconv.Atoi(revenueStr[len(revenueStr)-2:])
	c := &Currency{
		Dollars: currencyInts,
		Cents:   currencyCents,
	}
	return c
}

func (c *Currency) ToString() string {
	centsStr := "00"
	if c.Cents > 9 {
		centsStr = strconv.Itoa(c.Cents)
	} else {
		centsStr = "0" + strconv.Itoa(c.Cents)
	}
	revenueStr := "$" + strconv.Itoa(c.Dollars) + "." + centsStr
	return revenueStr
}

func (c *Currency) ToAirtableString() string {
	centsStr := "00"
	if c.Cents > 9 {
		centsStr = strconv.Itoa(c.Cents)
	} else {
		centsStr = "0" + strconv.Itoa(c.Cents)
	}
	revenueStr := strconv.Itoa(c.Dollars) + "." + centsStr
	return revenueStr
}

func CurrencyFromFloat(curr float64) *Currency {
	c := &Currency{
		Dollars: int(curr),
		Cents:   int((curr-math.Floor(curr))*100 + 0.5),
	}
	return c
}

func (c *Currency) ToFloat() float64 {
	var curr float64 = float64(c.Dollars)
	curr += (float64(c.Cents) / 100)
	return curr
}

func (c *Currency) ToCurrencyInt() int64 {
	var curr int64 = int64(c.ToFloat() * 100)
	return curr
}

func GetConstant(constantName string) (*Constant, error) {
	if defaultCache != nil {
		tempC := Constant{Name: constantName}
		if c, found := defaultCache.Get(tempC.cacheKey()); found {
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

func (c *Constant) UpdateConstantValue(value int) error {
	c.Value = value

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: c.AirtableID,
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
		defaultCache.Delete(c.cacheKey())
	}

	return nil
}

func GetAggregation(aggName string) (*Aggregation, error) {
	if defaultCache != nil {
		tempA := Aggregation{Name: aggName}
		if a, found := defaultCache.Get(tempA.cacheKey()); found {
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

	revenueStr := toStr(rec.Fields[fields.Revenue])[1:]
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

func (a *Aggregation) UpdateAggregation(quantity int, revenue int) error {
	a.Quantity = quantity
	a.Revenue = revenue

	revenueStr := "$" + strconv.Itoa(revenue/100) + "." + strconv.Itoa(revenue%100)

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: a.AirtableID,
			Fields: map[string]interface{}{
				fields.Quantity: a.Quantity,
				fields.Revenue:  revenueStr,
			},
		}},
	}

	_, err := aggregationsTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating aggregation")
	}

	if defaultCache != nil {
		defaultCache.Delete(a.cacheKey())
	}

	return nil
}

func (a *Aggregation) UpdateAggregationFromOrder(order *Order) error {
	if a.Name == fields.TotalTicketsSold || a.Name == fields.SoftLaunchSold {
		a.Quantity += order.TotalTickets
		a.Revenue += int(order.Total.ToFloat()) - order.Donation
	} else if a.Name == fields.CabinSold {
		a.Quantity += order.AdultCabin + order.ChildCabin + order.ToddlerCabin
		a.Revenue += (order.AdultCabin * 590) + (order.ChildCabin * 380)
	} else if a.Name == fields.TentSold {
		a.Quantity += order.AdultTent + order.ChildTent + order.ToddlerTent
		a.Revenue += (order.AdultTent * 420) + (order.ChildTent * 210)
	} else if a.Name == fields.SatSold {
		a.Quantity += order.AdultSat + order.ChildSat + order.ToddlerSat
		a.Revenue += (order.AdultSat * 140) + (order.ChildSat * 70)
	} else if a.Name == fields.AdultSold {
		a.Quantity += order.AdultCabin + order.AdultTent + order.AdultSat
		a.Revenue += (order.AdultCabin * 590) + (order.AdultTent * 420) + (order.AdultSat * 140)
	} else if a.Name == fields.ChildSold {
		a.Quantity += order.ChildCabin + order.ChildTent + order.ChildSat
		a.Revenue += (order.ChildCabin * 380) + (order.ChildTent * 210) + (order.ChildSat * 70)
	} else if a.Name == fields.DonationsRecv {
		if order.Donation > 0 {
			a.Quantity += 1
			a.Revenue += order.Donation
		}
	} else {
		return errors.New("No such aggregation")
	}

	revenueStr := "$" + strconv.Itoa(a.Revenue/100) + "." + strconv.Itoa(a.Revenue%100)

	r := &airtable.Records{
		Records: []*airtable.Record{{
			ID: a.AirtableID,
			Fields: map[string]interface{}{
				fields.Quantity: a.Quantity,
				fields.Revenue:  revenueStr,
			},
		}},
	}

	_, err := aggregationsTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating aggregation")
	}

	if defaultCache != nil {
		defaultCache.Delete(a.cacheKey())
	}

	return nil
}

func (a *Aggregation) MakeUpdatedRecord(order *Order) *airtable.Record {
	ticketTotal := int((order.Total.ToFloat()-order.ProcessingFee.ToFloat()-float64(order.Donation))*100 + 0.5)
	donationFee := int(float64(order.Donation)*stripeFee*100 + 0.5)
	if a.Name == fields.TotalTicketsSold || a.Name == fields.SoftLaunchSold {
		a.Quantity += order.TotalTickets
		a.Revenue += ticketTotal
	} else if a.Name == fields.CabinSold {
		a.Quantity += order.AdultCabin + order.ChildCabin + order.ToddlerCabin
		a.Revenue += ((order.AdultCabin * 590) + (order.ChildCabin * 380)) * 100
	} else if a.Name == fields.TentSold {
		a.Quantity += order.AdultTent + order.ChildTent + order.ToddlerTent
		a.Revenue += ((order.AdultTent * 420) + (order.ChildTent * 210)) * 100
	} else if a.Name == fields.SatSold {
		a.Quantity += order.AdultSat + order.ChildSat + order.ToddlerSat
		a.Revenue += ((order.AdultSat * 140) + (order.ChildSat * 70)) * 100
	} else if a.Name == fields.AdultSold {
		a.Quantity += order.AdultCabin + order.AdultTent + order.AdultSat
		a.Revenue += ((order.AdultCabin * 590) + (order.AdultTent * 420) + (order.AdultSat * 140)) * 100
	} else if a.Name == fields.ChildSold {
		a.Quantity += order.ChildCabin + order.ChildTent + order.ChildSat
		a.Revenue += ((order.ChildCabin * 380) + (order.ChildTent * 210) + (order.ChildSat * 70)) * 100
	} else if a.Name == fields.ToddlerSold {
		a.Quantity += order.ToddlerCabin + order.ToddlerTent + order.ToddlerSat
	} else if a.Name == fields.DonationsRecv {
		if order.Donation > 0 {
			a.Quantity += 1
			a.Revenue += (order.Donation*100 - donationFee)
		}
	} else if a.Name == fields.FullSold {
		a.Quantity += order.AdultCabin + order.AdultTent + order.ChildCabin + order.ChildTent + order.ToddlerCabin + order.ToddlerTent
		a.Revenue += ticketTotal
	}

	cents := a.Revenue % 100
	centsStr := "00"
	if cents < 10 {
		centsStr = "0" + strconv.Itoa(cents)
	} else {
		centsStr = strconv.Itoa(cents)
	}
	// "$" +
	revenueStr := strconv.Itoa(a.Revenue/100) + "." + centsStr
	revenueFloat, err := strconv.ParseFloat(revenueStr, 64)
	if err != nil {
		log.Errorf("Error converting revenue %s %v", revenueStr, err)
	}

	r := &airtable.Record{
		ID: a.AirtableID,
		Fields: map[string]interface{}{
			fields.Quantity: a.Quantity,
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

	log.Debugf("%v", r)
	_, err = aggregationsTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "updating aggregations")
	}

	return nil
}

func (c *Constant) cacheKey() string    { return "cons-" + c.Name }
func (a *Aggregation) cacheKey() string { return "agg-" + a.Name }
