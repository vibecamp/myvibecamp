package db

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mehanizm/airtable"
	"github.com/cockroachdb/errors"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
)

const checked = "checked"

var ErrNoRecords   = fmt.Errorf("no records found")
var ErrManyRecords = fmt.Errorf("multiple records for value")

var client *airtable.Client

var defaultTable *airtable.Table

var aggregationsTable *airtable.Table
var chaosModeUsersTable *airtable.Table
var constantsTable *airtable.Table
var ordersTable *airtable.Table
var productsTable *airtable.Table
var softLaunchUsersTable *airtable.Table
var sponsorshipTable *airtable.Table
var usersTable *airtable.Table

var defaultCache *cache.Cache

// TODO: Consider using a config struct where env vars are passed in as
// fields on the struct. Maybe in a map? This doesn't feel like it should need
// to know anything about env vars.
func Init(cacheTime time.Duration) {
	var (
		airtableAPIKey           = os.Getenv("AIRTABLE_API_KEY")
		airtableBaseID2022       = os.Getenv("AIRTABLE_BASE_ID_2022")
		defaultTableName         = os.Getenv("AIRTABLE_DEFAULT_TABLE")
		airtableBaseID2023       = os.Getenv("AIRTABLE_BASE_ID_2023")
		aggregationsTableName    = os.Getenv("AIRTABLE_AGGREGATIONS_TABLE")
		chaosModeUsersTableName  = os.Getenv("AIRTABLE_CHAOS_MODE_USERS_TABLE")
		constantsTableName       = os.Getenv("AIRTABLE_CONSTANTS_TABLE")
		ordersTableName          = os.Getenv("AIRTABLE_ORDERS_TABLE")
		productsTableName        = os.Getenv("AIRTABLE_PRODUCTS_TABLE")
		softLaunchUsersTableName = os.Getenv("AIRTABLE_SOFT_LAUNCH_USERS_TABLE")
		sponsorshipsTableName    = os.Getenv("AIRTABKE_SPONSORSHIPS_TABLE")
		usersTableName           = os.Getenv("AIRTABLE_USERS_TABLE")
	)

	client = airtable.NewClient(airtableAPIKey)

	defaultTable = client.GetTable(airtableBaseID2022, defaultTableName)

	aggregationsTable    = client.GetTable(airtableBaseID2023, aggregationsTableName)
	chaosModeUsersTable  = client.GetTable(airtableBaseID2023, chaosModeUsersTableName)
	constantsTable       = client.GetTable(airtableBaseID2023, constantsTableName)
	ordersTable          = client.GetTable(airtableBaseID2023, ordersTableName)
	productsTable        = client.GetTable(airtableBaseID2023, productsTableName)
	softLaunchUsersTable = client.GetTable(airtableBaseID2023, softLaunchUsersTableName)
	sponsorshipTable     = client.GetTable(airtableBaseID2023, sponsorshipsTableName)
	usersTable           = client.GetTable(airtableBaseID2023, usersTableName)

	defaultCache = cache.New(cacheTime, 1 * time.Hour)
}

func query(table *airtable.Table, field, value string, returnFields ...string) (*airtable.Records, error) {
	filterFormula := fmt.Sprintf(`{%s}="%s"`, field, strings.ReplaceAll(value, `"`, `\"`))
	log.Debugf(`airtable query: %s `, filterFormula)
	records, err := table.GetRecords().
		// FromView("view_1").
		WithFilterFormula(filterFormula).
		// WithSort(sortQuery1, sortQuery2).
		ReturnFields(returnFields...).
		InStringFormat("US/Eastern", "en").
		Do()

	return records, errors.Wrap(err, "")
}
