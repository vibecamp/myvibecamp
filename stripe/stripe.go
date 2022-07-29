package stripe 

import (
  "bytes"
  "encoding/json"
  "io"
  "log"
  "net/http"
//  "os"
  "fmt"
  "strings"

  "github.com/lyoshenka/vibedata/db"

  "github.com/gin-contrib/sessions"
  "github.com/cockroachdb/errors"
  "github.com/stripe/stripe-go/v72"
  "github.com/stripe/stripe-go/v72/paymentintent"
  "github.com/gin-gonic/gin"
  "github.com/google/uuid"
)

type item struct {
  id string
  quantity int
  amount int
}

// this could be put in the DB but should it be? hmm
var ticketPrices = map[string] int {"adult-cabin":590,"adult-tent":420,"child-cabin":380,"child-tent":210,"toddler-cabin":0,"toddler-tent":0}

func calculateCartInfo(items []item, ticketLimit int) (*db.Order, error) {
  order := &db.Order{}
  order.Total = 0
  order.TotalTickets = 0
  order.OrderID = ""
  order.UserName = ""
  order.StripeID = ""
  order.PaymentStatus = ""
  order.AirtableID = ""
  for _, element := range items {
	  if element.id == "donation" && element.quantity > 0 && element.amount > 0 {
		order.Total += element.amount
		order.Donation = element.amount
	  } else if element.quantity > 0 {
		if (element.quantity > ticketLimit || !strings.HasPrefix(element.id, "adult")) {
			return nil, errors.New("Exceeded soft launch ticket limit")
		}	
		
		 price, ok1 := ticketPrices[element.id]
		 if ok1 {
			order.Total += (price * element.quantity)
			order.TotalTickets += element.quantity

			if element.id == "adult-cabin" {
				order.AdultCabin = element.quantity
			} else if element.id == "adult-tent" {
				order.AdultTent = element.quantity
			} else if element.id == "child-cabin" {
				order.ChildCabin = element.quantity
			} else if element.id == "child-tent" {
				order.ChildTent = element.quantity
			} else if element.id == "toddler-cabin" {
				order.ToddlerCabin = element.quantity
			} else if element.id == "toddler-tent" {
				order.ToddlerTent = element.quantity
			}
		 }
	  }
  }

  return order, nil
}

type Session struct {
	UserName	string
	TwitterName string
	TwitterID   string
}

const sessionKey = "s"

func GetSession(c *gin.Context) *Session {
	defaultSession := sessions.Default(c)
	s := defaultSession.Get(sessionKey)
	if s == nil {
		return new(Session)
	}

	ss := s.(Session)
	return &ss
}

func HandleCreatePaymentIntent(c *gin.Context) {
	var w http.ResponseWriter = c.Writer
	var r *http.Request = c.Request
	if r.Method != "POST" {
	  http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	  return
	}

	session := GetSession(c)
	if session == nil || session.UserName == "" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	var req struct {
	  Items []item `json:"items"`
	}
  
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  log.Printf("json.NewDecoder.Decode: %v", err)
	  return
	}

	user, err := db.GetSoftLaunchUser(session.UserName)
	newUser, err := db.GetUser(session.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// ticketLimit := user.ticketLimit or something
	// then loop through items to find adult tickets & make sure they're within allotment
	
	// need to store this for it to work correctly
	// think it should only be stored after succesful api call i think
	// idempotencyKey := uuid.NewString()

	order, err := calculateCartInfo(req.Items, user.TicketLimit)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	order.UserName = user.UserName
	order.OrderID = uuid.NewString()
  
	// stripe.Key = os.Getenv("STRIPE_API_KEY")
  	 stripe.Key = "sk_test_4eC39HqLyjWDarjtT1zdp7dc"
	// Create a PaymentIntent with amount and currency
	params := &stripe.PaymentIntentParams{
	  Amount:   stripe.Int64(int64(order.Total) * 100),
	  Currency: stripe.String(string(stripe.CurrencyEUR)),
	  AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
		Enabled: stripe.Bool(true),
	  },
	  // change when we do hard launch or if we want to add vibecamp2/other name
	  StatementDescriptor: stripe.String("tickets to vibecamp"),
	  Description: stripe.String(fmt.Sprintf("%d tickets to vibecamp", order.TotalTickets)),
	}

	// create an (unpaid) order here? can track orders separate from payments that way ig
	// or before the payment intent creation, then allowing us to add id to payment intent
	// as metadata
	// why not put all the order data into the metadata? hmm
	// can use orders to store customer info away from stripe tho since we dont have uids

	params.AddMetadata("orderId", order.OrderID)
  
	pi, err := paymentintent.New(params)
	log.Printf("pi.New: %v", pi.ClientSecret)
  
	if err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  log.Printf("pi.New: %v", err)
	  return
	}

	order.StripeID = pi.ID

	err = order.CreateOrder()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("order.CreateOrder: %v", err)
		return
	}

	err = newUser.UpdateOrderID(order.OrderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("user.UpdateOrderID: %v", err)
		return
	}

	writeJSON(w, struct {
	  ClientSecret string `json:"clientSecret"`
	}{
	  ClientSecret: pi.ClientSecret,
	})
  }

func writeJSON(w http.ResponseWriter, v interface{}) {
  var buf bytes.Buffer
  if err := json.NewEncoder(&buf).Encode(v); err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    log.Printf("json.NewEncoder.Encode: %v", err)
    return
  }
  w.Header().Set("Content-Type", "application/json")
  if _, err := io.Copy(w, &buf); err != nil {
    log.Printf("io.Copy: %v", err)
    return
  }
}