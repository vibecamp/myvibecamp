package db

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/lyoshenka/vibedata/fields"
)

func GetCabinsForBadgeGenerator() (map[string]string, error) {
	filterFormula := fmt.Sprintf(`AND({%s}="yes",NOT({%s}=BLANK()))`, fields.Badge, fields.Cabin)
	response, err := defaultTable.GetRecords().
		//FromView("view_1").
		WithFilterFormula(filterFormula).
		//WithSort(sortQuery1, sortQuery2).
		ReturnFields(fields.TwitterName, fields.Cabin).
		InStringFormat("US/Eastern", "en").
		Do()

	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	cabins := make(map[string]string, len(response.Records))
	for _, r := range response.Records {
		cabins[toStr(r.Fields[fields.TwitterName])] = toStr(r.Fields[fields.Cabin])
	}

	return cabins, nil
}
