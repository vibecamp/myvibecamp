package db

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/mehanizm/airtable"
	log "github.com/sirupsen/logrus"
	"github.com/vibecamp/myvibecamp/fields"
)

type Item struct {
	Id       string `json:"id"`
	Quantity int    `json:"quantity"`
	Amount   int    `json:"amount"`
}

var CartFields = map[string]string{
	fields.AdultCabin:   "adult-cabin",
	fields.AdultTent:    "adult-tent",
	fields.AdultSat:     "adult-sat",
	fields.ChildCabin:   "child-cabin",
	fields.ChildTent:    "child-tent",
	fields.ChildSat:     "child-sat",
	fields.ToddlerCabin: "toddler-cabin",
	fields.ToddlerTent:  "toddler-tent",
	fields.ToddlerSat:   "toddler-sat",
	fields.Donation:     "donation",
}

func (o *Order) ReplaceCart(a *Order) error {
	a.AirtableID = o.AirtableID
	a.StripeID = o.StripeID
	a.OrderID = o.OrderID
	a.UserName = o.UserName
	r := &airtable.Records{
		Records: []*airtable.Record{
			{
				ID: a.AirtableID,
				Fields: map[string]interface{}{
					fields.Total:         a.Total.ToFloat(),
					fields.ProcessingFee: a.ProcessingFee.ToFloat(),
					fields.TotalTickets:  a.TotalTickets,
					fields.AdultCabin:    a.AdultCabin,
					fields.AdultTent:     a.AdultTent,
					fields.AdultSat:      a.AdultSat,
					fields.ChildCabin:    a.ChildCabin,
					fields.ChildTent:     a.ChildTent,
					fields.ChildSat:      a.ChildSat,
					fields.ToddlerCabin:  a.ToddlerCabin,
					fields.ToddlerTent:   a.ToddlerTent,
					fields.ToddlerSat:    a.ToddlerSat,
					fields.Donation:      a.Donation,
				},
			},
		},
	}

	_, err := ordersTable.UpdateRecordsPartial(r)
	if err != nil {
		return errors.Wrap(err, "Error updating order info - contact @orb_net if this persists")
	}

	if defaultCache != nil {
		defaultCache.Delete(o.cacheKey())
	}
	return nil
}

func (o *Order) IsEqual(a *Order) bool {
	if o.Total.ToString() != a.Total.ToString() {
		return false
	}

	oCart := o.toCart()
	aCart := a.toCart()

	for item := range CartFields {
		if oCart[item] != aCart[item] {
			return false
		}
	}

	return true
}

func (o *Order) toCart() map[string]int {
	return map[string]int{
		"adult-cabin":   o.AdultCabin,
		"adult-tent":    o.AdultTent,
		"adult-sat":     o.AdultSat,
		"child-cabin":   o.ChildCabin,
		"child-tent":    o.ChildTent,
		"child-sat":     o.ChildSat,
		"toddler-cabin": o.ToddlerCabin,
		"toddler-tent":  o.ToddlerTent,
		"toddler-sat":   o.ToddlerSat,
		"donation":      o.Donation,
	}
}

func toInt(i interface{}) int {
	if i == nil {
		return 0
	}
	num, _ := strconv.Atoi(i.(string))
	return num
}

func (o *Order) cacheKey() string { return o.OrderID }

type ItemType int64

const (
	Undefined ItemType = iota
	AdultCabin
	AdultTent
	AdultSat
	ChildCabin
	ChildTent
	ChildSat
	ToddlerCabin
	ToddlerTent
	ToddlerSat
	Donation
	SatToTentUpgrade
	TentToCabinUpgrade
)

func (i ItemType) String() string {
	switch i {
	case AdultCabin:
		return "adult-cabin"
	case AdultTent:
		return "adult-tent"
	case AdultSat:
		return "adult-sat"
	case ChildCabin:
		return "child-cabin"
	case ChildTent:
		return "child-tent"
	case ChildSat:
		return "child-sat"
	case ToddlerCabin:
		return "toddler-cabin"
	case ToddlerTent:
		return "toddler-tent"
	case ToddlerSat:
		return "toddler-sat"
	case Donation:
		return "donation"
	case SatToTentUpgrade:
		return "sat-tent-upgrade"
	case TentToCabinUpgrade:
		return "tent-cabin-upgrade"
	}

	return "unknown"
}

func MakeStringCart(i []Item) string {
	cart := ""
	for idx, item := range i {
		if idx != 0 {
			cart += ","
		}

		if item.Id == Donation.String() && item.Amount > 0 && item.Quantity > 0 {
			cart += fmt.Sprintf("%s %d", item.Id, item.Amount)
		} else if item.Quantity > 0 {
			cart += fmt.Sprintf("%s %d", item.Id, item.Quantity)
		}
	}
	return cart
}

func StringCartToItem(cart string) []Item {
	cartItems := strings.Split(cart, ",")
	items := []Item{}

	for _, item := range cartItems {
		splitItem := strings.Split(item, " ")

		if len(splitItem) == 1 {
			break
		}

		numVal, err := strconv.Atoi(splitItem[1])

		if err != nil {
			log.Error("Atoi error: %v", err)
		}

		newItem := Item{
			Id:       splitItem[0],
			Quantity: 0,
			Amount:   0,
		}

		if newItem.Id == Donation.String() {
			newItem.Amount = numVal
		} else {
			newItem.Quantity = numVal
		}

		items = append(items, newItem)
	}

	return items
}
