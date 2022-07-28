package stripe 

import (
  "bytes"
  "encoding/json"
  "io"
  "log"
  "net/http"
  "os"
  "fmt"
  "strings"
  "uuid"

  "github.com/stripe/stripe-go/v72"
  "github.com/stripe/stripe-go/v72/paymentintent"
  "github.com/gin-gonic/gin"
)

type item struct {
  id string
  quantity int
  amount int
}

type CartInfo struct {
	total int64
	stripeTotal int64
	totalTickets int
	scholarshipTickets int
}

// this could be put in the DB but should it be? hmm
ticketPrices := map[string] int {"adult-cabin":590,"adult-tent":420,"child-cabin":380,"child-tent":210,"toddler-cabin":0,"toddler-tent":0}

func calculateCartInfo(items []item) CartInfo {
  var cart CartInfo
  cart.total = 0
  cart.totalTickets = 0
  cart.scholarshipTickets = 0
  for _, element := range items {
	  if element.id == "donation" && element.quantity > 0 && element.amount > 0 {
		cart.total += element.amount
		cart.scholarshipTickets = element.amount / 420
	  } else if element.quantity > 0 && (element.quantity < 2 || !strings.HasPrefix(element.id, "adult")) {
		// this should be changed to use the limit on a person's tickets - generally will be one but need a way to change to 2
		// i should do this below instead, in handle payment intent so that it can reject it before calling a bunch of other funcs
		// then can just add normally here
		 price, ok1 := ticketPrices[element.id]
		 if ok1 {
			cart.total += (price * element.quantity)
			cart.totalTickets += element.quantity
		 }
	  }
  }

  // stripe takes prices in cents
  cart.stripeTotal = cart.total * 100

  return cart
}

// calculate the total price for an order
func calculateOrderAmount(items []item) int64 {
  var total int64 = 0
  for _, element := range items {
	  if element.id == "donation" && element.quantity > 0 && element.amount > 0 {
		total += element.amount
	  } else if element.quantity > 0 && element.quantity < 2 {
		// this should be changed to use the limit on a person's tickets - generally will be one but need a way to change to 2
		 price, ok1 := ticketPrices[element.id]
		 if ok1 {
			total += (price * element.quantity)
		 }
	  }
  }
  return total
}


func HandleCreatePaymentIntent(c *gin.Context) {
	var w http.ResponseWriter = c.Writer
	var r *http.Request = c.Request
	if r.Method != "POST" {
	  http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	  return
	}

	session := GetSession(c)
	if !session.SignedIn() {
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

	user, err := db.GetUser(session.TwitterName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// ticketLimit := user.ticketLimit or something
	// then loop through items to find adult tickets & make sure they're within allotment
	
	// need to store this for it to work correctly
	// think it should only be stored after succesful api call i think
	// idempotencyKey := uuid.NewString()

	var cart CartInfo = calculateCartInfo(items)
  
	stripe.Key = os.Getenv("STRIPE_API_KEY")
  	// stripe.Key = "sk_test_4eC39HqLyjWDarjtT1zdp7dc"
	// Create a PaymentIntent with amount and currency
	params := &stripe.PaymentIntentParams{
	  Amount:   stripe.Int64(cart.stripeTotal),
	  Currency: stripe.String(string(stripe.CurrencyEUR)),
	  AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
		Enabled: stripe.Bool(true),
	  },
	  // change when we do hard launch or if we want to add vibecamp2/other name
	  StatementDescriptor: stripe.String("tickets to vibecamp"),
	  Descriptor: stripe.String(fmt.Sprintf("%d tickets to vibecamp", cart.totalTickets))
	}

	// create an (unpaid) order here? can track orders separate from payments that way ig
	// or before the payment intent creation, then allowing us to add id to payment intent
	// as metadata
	// why not put all the order data into the metadata? hmm
	// can use orders to store customer info away from stripe tho since we dont have uids

	orderId := uuid.NewString()
	params.AddMetaData("orderId", orderId)
	params.AddMetaData("cart", json.Marshal(req))

  
	pi, err := paymentintent.New(params)
	log.Printf("pi.New: %v", pi.ClientSecret)
  
	if err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  log.Printf("pi.New: %v", err)
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