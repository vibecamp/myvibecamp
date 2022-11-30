package db

import (
	"math"
	"strconv"
	"strings"
)

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
