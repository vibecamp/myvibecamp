package stripe 

import (
  "bytes"
  "encoding/json"
  "io"
  "io/ioutil"
  "net/http"
  "fmt"
  "strings"

  "github.com/lyoshenka/vibedata/db"

  "github.com/cockroachdb/errors"
  "github.com/stripe/stripe-go/v72"
  "github.com/stripe/stripe-go/v72/paymentintent"
  "github.com/stripe/stripe-go/v72/webhook"
  "github.com/gin-gonic/gin"
  "github.com/google/uuid"
  log "github.com/sirupsen/logrus"
)

type Item struct {
  Id string `json:"id"`
  Quantity int `json:"quantity"`
  Amount int `json:"amount"`
}

// this could be put in the DB but should it be? hmm
var ticketPrices = map[string] int {"adult-cabin":590,"adult-tent":420,"child-cabin":380,"child-tent":210,"toddler-cabin":0,"toddler-tent":0}

func Init(key string) {
	stripe.Key = key
}

func calculateCartInfo(items []Item, ticketLimit int) (*db.Order, error) {
  order := &db.Order{}
  order.Total = 0
  order.TotalTickets = 0
  order.OrderID = ""
  order.UserName = ""
  order.StripeID = ""
  order.PaymentStatus = ""
  order.AirtableID = ""
  for _, element := range items {
	  if element.Id == "donation" && element.Quantity > 0 && element.Amount > 0 {
		order.Total += element.Amount
		order.Donation = element.Amount
	  } else if element.Quantity > 0 {
		if (element.Quantity > ticketLimit && strings.HasPrefix(element.Id, "adult")) {
			return nil, errors.New("Exceeded soft launch ticket limit")
		}	
		
		 price, ok1 := ticketPrices[element.Id]
		 if ok1 {
			order.Total += (price * element.Quantity)
			order.TotalTickets += element.Quantity

			if element.Id == "adult-cabin" {
				order.AdultCabin = element.Quantity
			} else if element.Id == "adult-tent" {
				order.AdultTent = element.Quantity
			} else if element.Id == "child-cabin" {
				order.ChildCabin = element.Quantity
			} else if element.Id == "child-tent" {
				order.ChildTent = element.Quantity
			} else if element.Id == "toddler-cabin" {
				order.ToddlerCabin = element.Quantity
			} else if element.Id == "toddler-tent" {
				order.ToddlerTent = element.Quantity
			}
		 }
	  }
  }

  return order, nil
}

func HandleCreatePaymentIntent(c *gin.Context) {
	var w http.ResponseWriter = c.Writer
	var r *http.Request = c.Request
	if r.Method != "POST" {
	  http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	  return
	}

	var req struct {
	  Items []Item `json:"items"`
	  UserName string `json:"username"`
	}
  
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  log.Printf("json.NewDecoder.Decode: %v", err)
	  return
	}

	user, err := db.GetSoftLaunchUser(req.UserName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	newUser, err := db.GetUser(req.UserName)
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

func HandleStripeWebhook(c *gin.Context) {
  var w http.ResponseWriter = c.Writer
  var req *http.Request = c.Request
  const MaxBodyBytes = int64(65536)
  req.Body = http.MaxBytesReader(w, req.Body, MaxBodyBytes)
  payload, err := ioutil.ReadAll(req.Body)
  if err != nil {
    log.Errorf("Error reading request body: %v\n", err)
    w.WriteHeader(http.StatusServiceUnavailable)
    return
  }

  event := stripe.Event{}

  if err := json.Unmarshal(payload, &event); err != nil {
    log.Errorf("⚠️  Webhook error while parsing basic request. %v\n", err.Error())
    w.WriteHeader(http.StatusBadRequest)
    return
  }

  // Replace this endpoint secret with your endpoint's unique secret
  // If you are testing with the CLI, find the secret by running 'stripe listen'
  // If you are using an endpoint defined with the API or dashboard, look in your webhook settings
  // at https://dashboard.stripe.com/webhooks
  endpointSecret := "whsec_..."
  signatureHeader := req.Header.Get("Stripe-Signature")
  event, err = webhook.ConstructEvent(payload, signatureHeader, endpointSecret)
  if err != nil {
    log.Errorf("⚠️  Webhook signature verification failed. %v\n", err)
    w.WriteHeader(http.StatusBadRequest) // Return a 400 error on a bad signature
    return
  }
  // Unmarshal the event data into an appropriate struct depending on its Type
  switch event.Type {
  case "payment_intent.succeeded":
    var paymentIntent stripe.PaymentIntent
    err := json.Unmarshal(event.Data.Raw, &paymentIntent)
    if err != nil {
      log.Errorf("Error parsing webhook JSON: %v\n", err)
      w.WriteHeader(http.StatusBadRequest)
      return
    }
    log.Printf("Successful payment for %d.", paymentIntent.Amount)
    // Then define and call a func to handle the successful payment intent.
    // handlePaymentIntentSucceeded(paymentIntent)

	// update order in db to mark as successful payment
	order,err := db.GetOrderByPaymentID(paymentIntent.ID)
	if err != nil {
		log.Errorf("error getting order by payment id: %v\n", err)
		return
	}

	err = order.UpdateOrderStatus("successful")
	if err != nil {
		log.Errorf("error updating order payment status: %v\n", err)
		return
	}
  case "payment_intent.processing":
    var paymentIntent stripe.PaymentIntent
    err := json.Unmarshal(event.Data.Raw, &paymentIntent)
    if err != nil {
      log.Errorf("Error parsing webhook JSON: %v\n", err)
      w.WriteHeader(http.StatusBadRequest)
      return
    }
    log.Printf("Processing payment for %d.", paymentIntent.Amount)

	// idk do nothing here?
	order,err := db.GetOrderByPaymentID(paymentIntent.ID)
	if err != nil {
		log.Errorf("error getting order by payment id: %v\n", err)
		return
	}

	err = order.UpdateOrderStatus("processing")
	if err != nil {
		log.Errorf("error updating order payment status: %v\n", err)
		return
	}

  case "payment_intent.failed":
    var paymentIntent stripe.PaymentIntent
    err := json.Unmarshal(event.Data.Raw, &paymentIntent)
    if err != nil {
      log.Errorf("Error parsing webhook JSON: %v\n", err)
      w.WriteHeader(http.StatusBadRequest)
      return
    }
    log.Printf("Failed payment for %d.", paymentIntent.Amount)

	// update db with failed order
	// update user to have 0 tickets in table? or just rm from table?
	order,err := db.GetOrderByPaymentID(paymentIntent.ID)
	if err != nil {
		log.Errorf("error getting order by payment id: %v\n", err)
		return
	}

	err = order.UpdateOrderStatus("failed")
	if err != nil {
		log.Errorf("error updating order payment status: %v\n", err)
		return
	}

  default:
    log.Errorf("Unhandled event type: %s\n", event.Type)
  }

  w.WriteHeader(http.StatusOK)
}