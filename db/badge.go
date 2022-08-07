package db

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/vibecamp/myvibecamp/fields"
)

func GetCabinsForBadgeGenerator() (map[string]string, error) {
	var cabins map[string]string
	filterFormula := fmt.Sprintf(`AND({%s}="yes",NOT({%s}=BLANK()))`, fields.Badge, fields.Cabin)
	offset := ""

	for {
		response, err := defaultTable.GetRecords().
			WithOffset(offset).
			//FromView("view_1").
			WithFilterFormula(filterFormula).
			//WithSort(sortQuery1, sortQuery2).
			ReturnFields(fields.TwitterName, fields.Cabin).
			InStringFormat("US/Eastern", "en").
			Do()

		if err != nil {
			return nil, errors.Wrap(err, "")
		}

		if cabins == nil {
			cabins = make(map[string]string, len(response.Records))
		}
		for _, r := range response.Records {
			cabins[toStr(r.Fields[fields.TwitterName])] = toStr(r.Fields[fields.Cabin])
		}

		if response.Offset == "" {
			break
		}
		offset = response.Offset
	}

	return cabins, nil
}
