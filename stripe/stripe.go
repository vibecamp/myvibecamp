package stripe 

import (
  "bytes"
  "encoding/json"
  "io"
  "log"
  "net/http"
  // "os"

  "github.com/stripe/stripe-go/v72"
  "github.com/stripe/stripe-go/v72/paymentintent"
  "github.com/gin-gonic/gin"
)

type item struct {
  id string
}

func calculateOrderAmount(items []item) int64 {
  // Replace this constant with a calculation of the order's amount
  // Calculate the order total on the server to prevent
  // people from directly manipulating the amount on the client
  var total = 0
  for index, element := range items {
	  if element.id == "vc-ticket" {
		  total = total + 300
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
  
	var req struct {
	  Items []item `json:"items"`
	}
  
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  log.Printf("json.NewDecoder.Decode: %v", err)
	  return
	}
  
	stripe.Key = os.Getenv("STRIPE_API_KEY")
  	// stripe.Key = "sk_test_4eC39HqLyjWDarjtT1zdp7dc"
	// Create a PaymentIntent with amount and currency
	params := &stripe.PaymentIntentParams{
	  Amount:   stripe.Int64(calculateOrderAmount(req.Items)),
	  Currency: stripe.String(string(stripe.CurrencyEUR)),
	  AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
		Enabled: stripe.Bool(true),
	  },
	}
  
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