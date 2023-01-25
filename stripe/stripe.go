package stripe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vibecamp/myvibecamp/db"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/paymentintent"
	"github.com/stripe/stripe-go/v72/webhook"
)

// this could be put in the DB but should it be? hmm
var ticketPrices = map[string]int{"adult-cabin": 590, "adult-tent": 420, "adult-sat": 140, "child-cabin": 380, "child-tent": 210, "child-sat": 70, "toddler-cabin": 0, "toddler-tent": 0, "toddler-sat": 0}
var stripeFeePercent float64 = 0.03
var webhookSecret = ""
var klaviyoKey = ""
var klaviyoListId = ""

func Init(key string, secret string, klaviyo string, klaviyoList string) {
	stripe.Key = key
	webhookSecret = secret
	klaviyoKey = klaviyo
	klaviyoListId = klaviyoList
}

func calculateCartInfo(items []db.Item, ticketLimit int) (*db.Order, error) {
	order := &db.Order{}
	order.TotalTickets = 0
	order.OrderID = ""
	order.UserName = ""
	order.StripeID = ""
	order.PaymentStatus = ""
	order.AirtableID = ""
	var ticketTotal float64 = 0
	for _, element := range items {
		if element.Id == "donation" && element.Quantity > 0 && element.Amount > 0 {
			order.Donation = element.Amount
		} else if element.Quantity > 0 {
			if element.Quantity > ticketLimit && strings.HasPrefix(element.Id, "adult") {
				return nil, errors.New("Exceeded soft launch ticket limit")
			}

			price, ok1 := ticketPrices[element.Id]
			if ok1 {
				if element.Id == "adult-tent" {
					ticketTotal += (float64(element.Quantity) * float64(420.69))
				} else {
					ticketTotal += float64(price * element.Quantity)
				}
				order.TotalTickets += element.Quantity

				if element.Id == "adult-cabin" {
					order.AdultCabin = element.Quantity
				} else if element.Id == "adult-tent" {
					order.AdultTent = element.Quantity
				} else if element.Id == "adult-sat" {
					order.AdultSat = element.Quantity
				} else if element.Id == "child-cabin" {
					order.ChildCabin = element.Quantity
				} else if element.Id == "child-tent" {
					order.ChildTent = element.Quantity
				} else if element.Id == "child-sat" {
					order.ChildSat = element.Quantity
				} else if element.Id == "toddler-cabin" {
					order.ToddlerCabin = element.Quantity
				} else if element.Id == "toddler-tent" {
					order.ToddlerTent = element.Quantity
				} else if element.Id == "toddler-sat" {
					order.ToddlerSat = element.Quantity
				}
			}
		}
	}

	var stripeFee float64 = ticketTotal * stripeFeePercent
	order.ProcessingFee = db.CurrencyFromFloat(stripeFee)
	order.Total = db.CurrencyFromFloat(float64(ticketTotal) + order.ProcessingFee.ToFloat() + float64(order.Donation))
	order.Date = time.Now().UTC().Format("2006-01-02 15:04")
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
		Items     []db.Item `json:"items"`
		UserName  string    `json:"username"`
		UserType  string    `json:"usertype"`
		OrderType string    `json:"ordertype"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Errorf("json.NewDecoder.Decode: %v", err)
		return
	}

	var ticketLimit int = 1
	var order *db.Order
	if req.UserType == "chaos" {
		chaosUser, err := db.GetChaosUser(req.UserName)
		if err != nil {
			log.Errorf("db.GetChaosUser: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ticketLimit = chaosUser.TicketLimit
	} else if req.UserType == "2022" {
		user, err := db.GetSoftLaunchUser(req.UserName)
		if err != nil {
			log.Errorf("db.GetSoftLaunchUser: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ticketLimit = user.TicketLimit
	} else {
		sponsoredUser, err := db.GetSponsorshipUser(req.UserName)
		if err != nil {
			log.Errorf("db.GetSponsoredUser: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		order = makeSponsoredOrder(*sponsoredUser)
	}

	newUser, err := db.GetUser(req.UserName)
	if err != nil {
		log.Errorf("db.GetUser: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if order == nil {
		order, err = calculateCartInfo(req.Items, ticketLimit)
		if err != nil {
			log.Errorf("stripe.calculateCartInfo: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	order.UserName = newUser.UserName

	var pi *stripe.PaymentIntent
	if newUser.OrderID != "" {
		dbOrder, err := db.GetOrder(newUser.OrderID)

		if err != nil {
			order.OrderID = newUser.OrderID
			pi, err = handleNewOrder(order, newUser)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if dbOrder != nil {
			// if it's successful redirect
			if dbOrder.PaymentStatus == "success" || dbOrder.PaymentStatus == "processing" {
				c.Redirect(http.StatusFound, "/checkout-complete")
				return
			} else if dbOrder.PaymentStatus == "failed" {
				c.Redirect(http.StatusFound, "/checkout-failed")
				return
			}

			pi, err = handleDbOrder(dbOrder, order, newUser)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	} else {
		pi, err = handleNewOrder(order, newUser)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, struct {
		ClientSecret string  `json:"clientSecret"`
		Total        float64 `json:"total"`
		IntentId     string  `json:"intentId"`
	}{
		ClientSecret: pi.ClientSecret,
		Total:        order.Total.ToFloat(),
		IntentId:     pi.ID,
	})
}

func handleDbOrder(dbOrder *db.Order, order *db.Order, newUser *db.User) (*stripe.PaymentIntent, error) {
	pi, err := paymentintent.Get(dbOrder.StripeID, nil)
	if err != nil {
		log.Errorf("pi.Get %v", err)
		return nil, err
	}

	if pi.Status == "succeeded" {
		log.Errorf("pi already successful %v", pi.ID)
		return nil, errors.New("Payment already succeeded")
	}

	if dbOrder.IsEqual(order) {
		return pi, nil
	}

	err = dbOrder.ReplaceCart(order)
	if err != nil {
		log.Errorf("Error updating cart %v", err)
		return nil, err
	}

	log.Debugf("old order %+v", dbOrder)
	log.Debugf("new order %+v", order)
	params := &stripe.PaymentIntentParams{
		Amount:      stripe.Int64(order.Total.ToCurrencyInt()),
		Description: stripe.String(fmt.Sprintf("%d tickets to vibecamp", order.TotalTickets)),
	}
	pi, err = paymentintent.Update(order.StripeID, params)
	if err != nil {
		log.Errorf("pi.Update %v", err)
		return nil, err
	}

	return pi, nil
}

func handleNewOrder(order *db.Order, newUser *db.User) (*stripe.PaymentIntent, error) {
	if order.OrderID == "" {
		order.OrderID = uuid.NewString()
	}
	// Create a PaymentIntent with amount and currency
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(order.Total.ToCurrencyInt()),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		// change when we do hard launch or if we want to add vibecamp2/other name
		StatementDescriptor: stripe.String("tickets to vibecamp"),
		Description:         stripe.String(fmt.Sprintf("%d tickets to vibecamp", order.TotalTickets)),
	}

	params.AddMetadata("orderId", order.OrderID)
	// use order id as idempotency key
	params.SetIdempotencyKey(order.OrderID)

	pi, err := paymentintent.New(params)

	if err != nil {
		log.Errorf("pi.New: %v", err)
		return nil, err
	}

	order.StripeID = pi.ID

	err = order.CreateOrder()
	if err != nil {
		log.Errorf("order.CreateOrder: %v", err)
		return nil, err
	}

	err = newUser.UpdateOrderID(order.OrderID)
	if err != nil {
		log.Errorf("user.UpdateOrderID: %v", err)
		return nil, err
	}

	return pi, nil
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

func AddToKlaviyo(email string, admissionLevel string, donation string) error {
	klaviyoUrl := "https://a.klaviyo.com/api/v2/list/" + klaviyoListId + "/members?api_key=" + klaviyoKey

	admLvl := admissionLevel
	if admLvl == "Tent" {
		admLvl = "basic"
	} else if admLvl == "Saturday Night" {
		admLvl = "saturday"
	} else if admLvl == "Cabin" {
		admLvl = "cabin"
	}

	payload := strings.NewReader("{\"profiles\":[{\"email\":\"" + email + "\",\"Admission Level 2023\":\"" + admLvl + "\",\"2023 Donor\":\"" + donation + "\"}]}")

	req, _ := http.NewRequest("POST", klaviyoUrl, payload)

	req.Header.Add("Accept", "application/json")

	req.Header.Add("Content-Type", "application/json")

	_, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Errorf("Error adding email to klaviyo list %v %v", email, err)
		return err
	}

	// defer res.Body.Close()

	// body, _ := ioutil.ReadAll(res.Body)

	// fmt.Println(res)
	// fmt.Println(string(body))
	return nil
}

func HandleStripeWebhook(c *gin.Context) {
	var w http.ResponseWriter = c.Writer
	var req *http.Request = c.Request
	const MaxBodyBytes = int64(65536)
	req.Body = http.MaxBytesReader(w, req.Body, MaxBodyBytes)
	payload, err := io.ReadAll(req.Body)
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
	signatureHeader := req.Header.Get("Stripe-Signature")
	event, err = webhook.ConstructEvent(payload, signatureHeader, webhookSecret)
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
		order, err := db.GetOrderByPaymentID(paymentIntent.ID)
		if err != nil {
			log.Errorf("error getting order by payment id: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if order.PaymentStatus != "success" {
			err = order.UpdateOrderStatus("success")
			if err != nil {
				log.Errorf("error updating order payment status: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			user, err := db.GetUser(order.UserName)
			if err != nil {
				log.Errorf("error getting user %v\n", err)
				w.WriteHeader((http.StatusInternalServerError))
				return
			}

			// update aggregations
			err = db.UpdateAggregations(order, user.TicketPath == "2022 Attendee")
			if err != nil {
				log.Errorf("error updating aggregations %v\n", err)
				w.WriteHeader((http.StatusInternalServerError))
				return
			}

			err = user.UpdateTicketId(uuid.NewString())
			if err != nil {
				log.Errorf("error updating user ticket id %v\n", err)
				w.WriteHeader((http.StatusInternalServerError))
				return
			}

			if user.Email != "" {
				err = AddToKlaviyo(user.Email, user.AdmissionLevel, "$"+strconv.Itoa(order.Donation))
				if err != nil {
					log.Errorf("Error adding user to klaviyo %v\n", err)
				}
			} else {
				log.Debugf("User does not have an associated email")
			}
		} else {
			log.Debugf("Order %v already marked successful", order.OrderID)
		}
		w.WriteHeader(http.StatusOK)
		return
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
		order, err := db.GetOrderByPaymentID(paymentIntent.ID)
		if err != nil {
			log.Errorf("error getting order by payment id: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if order.PaymentStatus == "success" || order.PaymentStatus == "failed" {
			log.Info("Payment already updated to %v", order.PaymentStatus)
			w.WriteHeader(http.StatusOK)
			return
		}

		err = order.UpdateOrderStatus("processing")
		if err != nil {
			log.Errorf("error updating order payment status: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	case "payment_intent.payment_failed":
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
		order, err := db.GetOrderByPaymentID(paymentIntent.ID)
		if err != nil {
			log.Errorf("error getting order by payment id: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = order.UpdateOrderStatus("failed")
		if err != nil {
			log.Errorf("error updating order payment status: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	case "payment_intent.created":
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			log.Errorf("Error parsing webhook JSON: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Payment intent created %v", paymentIntent.ID)
		w.WriteHeader(http.StatusOK)
		return
	default:
		log.Errorf("Unhandled event type: %s\n", event.Type)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func makeSponsoredOrder(user db.SponsorshipUser) *db.Order {
	price := float64(420.69)
	if user.AdmissionLevel != "Tent" {
		price = float64(140)
	}
	subtotal := price - user.Discount.ToFloat()
	fee := subtotal * stripeFeePercent
	total := subtotal + fee
	order := &db.Order{
		OrderID:       "",
		UserName:      user.UserName,
		Total:         db.CurrencyFromFloat(total),
		ProcessingFee: db.CurrencyFromFloat(fee),
		TotalTickets:  1,
		AdultCabin:    0,
		AdultTent:     0,
		AdultSat:      0,
		ChildCabin:    0,
		ChildTent:     0,
		ChildSat:      0,
		ToddlerCabin:  0,
		ToddlerTent:   0,
		ToddlerSat:    0,
		Donation:      0,
		StripeID:      "",
		PaymentStatus: "",
		Date:          time.Now().UTC().Format("2006-01-02 15:04"),
		AirtableID:    "",
	}

	if user.AdmissionLevel == "Tent" {
		order.AdultTent = 1
	} else {
		order.AdultSat = 1
	}

	return order
}
