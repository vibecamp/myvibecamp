package db

import (
	"bytes"
	"encoding/gob"

	"github.com/mehanizm/airtable"
	"github.com/cockroachdb/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

type Constant struct {
	Name  string
	Value int

	AirtableID string
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

func (c *Constant) cacheKey() string    { return "cons-" + c.Name }
