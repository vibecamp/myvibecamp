package fields

const (
	// 2022 attendees table fields (reusing applicable ones in 2023)
	TwitterName             = "Twitter Name"
	TwitterNameClean        = "twitter clean"
	Name                    = "Name"
	Email                   = "Email"
	OrderNotes              = "Order Notes"
	AdmissionLevel          = "Admission Level"
	Cabin                   = "Cabin"
	CabinNumber             = "Cabin Number"
	TicketID                = "Ticket ID"
	OrderDate               = "Order Date"
	Phone                   = "Phone"
	StripeTX                = "Stripe Tx"
	TicketGroup             = "Ticket Group"
	Barcode                 = "Barcode"
	CheckedIn               = "Checked In"
	TransportTo             = "Transport To"
	TransportFrom           = "Transport From"
	BeddingRental           = "Bedding Rental"
	BeddingPaid             = "Bedding Paid"
	DepartureTime           = "Departure Time"
	ArrivalTime             = "Arrival Time"
	Badge                   = "Badge"
	BusToCamp               = "Bus To Camp"
	BusToAUS                = "Bus To AUS"
	Vegetarian              = "Vegetarian"
	GlutenFree              = "Gluten Free"
	LactoseIntolerant       = "Lactose Intolerant"
	FoodComments            = "Food Comments"
	POAP                    = "POAP"
	Cabin2022               = "2022 Cabin"
	SponsorshipConfirmation = "Sponsorship Confirmation"
	CabinNickname           = "Cabin Nickname (from Cabin)"
	Created                 = "Created"
	TentVillage             = "Tent Village"

	// Transport Form (also attendee table)
	AssistanceToCamp         = "Assistance To Camp"
	TravelMethod             = "Travel Method"
	FlyingInto               = "Flying Into"
	FlightArrivalTime        = "Flight Arrival Time"
	RVCamper                 = "RV/Camper"
	WrongCityRedirect        = "Wrong City Redirect"
	VehicleArrival           = "Vehicle Arrival"
	AssistanceFromCamp       = "Assistance From Camp"
	LeavingFrom              = "Leaving From"
	CityArrivalTime          = "City Arrival Time"
	EarlyArrival             = "Early Arrival"
	KnowHowTravelFromAirport = "Know How Travel From Airport"

	// Early Arrival Options
	TuesdayAfternoon = "Tuesday Afternoon"
	WedsMorning      = "Wednesday Morning"
	WedsAfternoon    = "Wednesday Afternoon"

	// Bedding Field Names
	SleepingBagRentals = "Sleeping Bag Rentals"
	SheetRentals       = "Sheet Rentals"
	PillowRentals      = "Pillow Rentals"

	// Orders for Bus & Bedding
	BusSpots        = "Bus Spots"
	BusToVibecamp   = "Bus to Vibecamp"
	BusFromVibecamp = "Bus from Vibecamp"
	SleepingBags    = "Sleeping Bags"
	SheetSets       = "Sheet Sets"
	Pillows         = "Pillows"

	// Bus Options
	BusSlot   = "Bus Slot"
	Purchased = "Purchased"
	Cap       = "Cap"

	// ticketing info from users table
	AdultCabinAttendees = "Adult Cabin Attendees on Ticket"
	AdultTentAttendees  = "Adult Tent Attendees on Ticket"
	ChildCabinAttendees = "Child Cabin Attendees on Ticket"
	ChildTentAttendees  = "Child Tent Attendees on Ticket"
	ToddlerAttendees    = "Toddler Attendees on Ticket"
	AdultSatAttendees   = "Adult Saturday Night Attendees on Ticket"

	// ticket paths
	Sponsorship  = "Sponsorship"
	Attendee2022 = "2022 Attendee"
	Application  = "Application"
	FCFS         = "FCFS"
	Lottery      = "Lottery"
	TicketSwap   = "Ticket Swap"
	Staff        = "Staff"
	Volunteer    = "Volunteer"
	LateApp      = "Late App"
	Comped       = "Comped"

	NA = "N/A"

	// Travel Methods
	Flying     = "Flying"
	OwnVehicle = "Own Vehicle"

	// Cities
	Baltimore = "Baltimore"
	Philly    = "Philadelphia"

	Other = "Other"

	Confirmed = "Confirmed"
	Denied    = "Denied"

	// UserName is new ones for 2023. for attendees & soft launch
	UserName    = "Username"
	DiscordName = "Discord Name"

	// Ticket Path indicates how an attendee got on the list (prev attendee, FCFS, etc)
	TicketPath = "Ticket Path"

	// TicketLimit is for all pre-purchase users table
	TicketLimit = "Ticket Limit"

	// ChaosMode users have Phase, indicating which phase of chaos mode they got in
	Phase = "Phase"

	// Sponsorship users have a discount - 0 if it's a full discount
	Discount = "Discount"

	// TicketType is for attendees (adult, child, or toddler)
	TicketType = "Ticket Type"
	OrderID    = "OrderID"
	Date       = "Date"

	// orders
	Total         = "Total"
	TotalTickets  = "Total Tickets"
	ProcessingFee = "Processing Fee"
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
	CardPacks     = "Card Packs"

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
	Sponsorships     = "Sponsorships"

	CheckinCount = "Checkin Count"
)
