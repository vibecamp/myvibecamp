package fields

type Order struct {
}

const (
	// 2022 attendees table fields (reusing applicable ones in 2023)
	TwitterName       = "Twitter Name"
	TwitterNameClean  = "twitter clean"
	Name              = "Name"
	Email             = "Email"
	OrderNotes        = "Order Notes"
	AdmissionLevel    = "Admission Level"
	Cabin             = "Cabin"
	CabinNumber       = "Cabin Number"
	TicketID          = "Ticket ID"
	OrderDate         = "Order Date"
	Phone             = "Phone"
	StripeTX          = "Stripe Tx"
	TicketGroup       = "Ticket Group"
	Barcode           = "Barcode"
	CheckedIn         = "Checked In"
	TransportTo       = "Transport To"
	TransportFrom     = "Transport From"
	BeddingRental     = "Bedding Rental"
	BeddingPaid       = "Bedding Paid"
	DepartureTime     = "Departure Time"
	ArrivalTime       = "Arrival Time"
	Badge             = "Badge"
	BusToCamp         = "Bus To Camp"
	BusToAUS          = "Bus To AUS"
	Vegetarian        = "Vegetarian"
	GlutenFree        = "Gluten Free"
	LactoseIntolerant = "Lactose Intolerant"
	FoodComments      = "Food Comments"
	POAP              = "POAP"

	// new ones for 2023
	// for attendees & soft launch
	UserName = "Username"

	// for soft launch users table
	TicketLimit = "Ticket Limit"

	// for attendees
	// adult child or toddler
	TicketType = "Ticket Type"
	OrderID    = "OrderID"

	// orders
	Total         = "Total"
	TotalTickets  = "Total Tickets"
	AdultCabin    = "Adult Cabin"
	AdultTent     = "Adult Tent"
	AdultSat      = "Adult Saturday Night"
	ChildCabin    = "Child Cabin"
	ChildTent     = "Child Tent"
	ChildSat      = "Child Saturday Night"
	ToddlerCabin  = "Toddler Cabin"
	ToddlerTent   = "Toddler Tent"
	ToddlerSat    = "Toddler Saturday Night"
	Donation      = "Donation Amount"
	PaymentID     = "PaymentIntentID"
	PaymentStatus = "Payment Status"

	// payments
	StripeID = "StripeID"
	Status   = "Status"

	// constants table
	Value = "Value"
	// record names
	SalesCap          = "Sales Cap"
	CabinCap          = "Cabin Cap"
	SoftCabinCap      = "Soft Launch Cabin Cap"
	SatCap            = "Saturday Night Cap"
	AdultCabinPrice   = "Adult Cabin Price"
	AdultTentPrice    = "Adult Tent Price"
	AdultSatPrice     = "Adult Saturday Price"
	ChildCabinPrice   = "Child Cabin Price"
	ChildTentPrice    = "Child Tent Price"
	ChildSatPrice     = "Child Saturday Price"
	ToddlerCabinPrice = "Toddler Cabin Price"
	ToddlerTentPrice  = "Toddler Tent Price"
	ToddlerSatPrice   = "Toddler Saturday Price"

	// aggregations
	Quantity = "Quantity"
	Revenue  = "Revenue"
	// record names
	TotalTicketsSold = "Total Tickets Sold"
	SoftLaunchSold   = "Soft Launch Tickets Sold"
	CabinSold        = "Cabin Tickets Sold"
	TentSold         = "Tent Tickets Sold"
	SatSold          = "Saturday Night Tickets Sold"
	FullSold         = "Full Tickets Sold"
	AdultSold        = "Adult Tickets Sold"
	ChildSold        = "Child Tickets Sold"
	ToddlerSold      = "Toddler Tickets Sold"
	DonationsRecv    = "Donations Received"
)
