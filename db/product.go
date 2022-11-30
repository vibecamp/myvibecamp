package db

import (
	"github.com/cockroachdb/errors"
	// log "github.com/sirupsen/logrus"
)

type Product struct {
	ProductID   string
	Name        string
	ProductType string
	Price       *Currency
	Enabled     bool

	AirtableID  string
}

func GetProduct(productID string) (*Product, error) {
	product, err := getProductByID(productID)
	if err != nil {
		if errors.Is(err, ErrNoRecords) {
			err = errors.New("Product not found.")
		}

		return nil, err
	} else if product == nil {
		return nil, errors.New("Product not found.")
	}

	return product, nil
}

func getProductByID(productID string) (*Product, error) {
	response, err := query(productsTable, "Product Id", productID)
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Records) == 0 {
		return nil, errors.Wrap(ErrNoRecords, "")
	}

	record := response.Records[0]

	product := &Product{
		AirtableID:  record.ID,
		ProductID:   toStr(record.Fields["Product ID"]),
		Name:        toStr(record.Fields["Name"]),
		ProductType: toStr(record.Fields["Product Type"]),
		Price:       CurrencyFromAirtableString(toStr(record.Fields["Price"])),
		Enabled:     record.Fields["Enabled"] == checked,
	}

	return product, nil
}