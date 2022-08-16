package stripe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/vibecamp/myvibecamp/db"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/paymentintent"
	"github.com/stripe/stripe-go/v72/webhook"
)

type Item struct {
	Id       string `json:"id"`
	Quantity int    `json:"quantity"`
	Amount   int    `json:"amount"`
}

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

func calculateCartInfo(items []Item, ticketLimit int) (*db.Order, error) {
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
	log.Debugf("%f", order.Total.ToFloat())
	log.Debugf("%f", stripeFee)
	log.Debugf("%v", order.Donation)

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
		Items    []Item `json:"items"`
		UserName string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Errorf("json.NewDecoder.Decode: %v", err)
		return
	}

	user, err := db.GetSoftLaunchUser(req.UserName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newUser, err := db.GetUser(req.UserName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// need to store this for it to work correctly
	// think it should only be stored after succesful api call i think
	// idempotencyKey := uuid.NewString()

	order, err := calculateCartInfo(req.Items, user.TicketLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	order.UserName = user.UserName
	order.OrderID = uuid.NewString()

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

	pi, err := paymentintent.New(params)
	log.Debugf("pi.New: %v", pi.ClientSecret)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Errorf("pi.New: %v", err)
		return
	}

	order.StripeID = pi.ID

	/* err = order.CreateOrder()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("order.CreateOrder: %v", err)
		return
	} */

	err = newUser.UpdateOrderID(order.OrderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Errorf("user.UpdateOrderID: %v", err)
		return
	}

	/** for testing aggregation updates without active webhook
	err = db.UpdateAggregations(order)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("UpdateAggregations: %v", err)
		return
	}
	*/

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

func HandlePaymentSent(c *gin.Context) {
	var w http.ResponseWriter = c.Writer
	var r *http.Request = c.Request
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Items    []Item `json:"items"`
		UserName string `json:"username"`
		IntentId string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Errorf("json.NewDecoder.Decode: %v", err)
		return
	}

	order, err := calculateCartInfo(req.Items, 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user, err := db.GetUser(req.UserName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	order.UserName = user.UserName
	order.OrderID = user.OrderID
	order.StripeID = req.IntentId
	order.PaymentStatus = "pending"

	err = order.CreateOrder()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("order.CreateOrder: %v", err)
		return
	}

	writeJSON(w, struct {
		OrderId string `json:"orderId"`
	}{
		OrderId: order.OrderID,
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

func AddToKlaviyo(email string, admissionLevel string, donation string) error {
	klaviyoUrl := "https://a.klaviyo.com/api/v2/list/" + klaviyoListId + "/members?api_key=" + klaviyoKey

	payload := strings.NewReader("{\"profiles\":[{\"email\":\"" + email + "\",\"Admission Level 2023\":\"" + admissionLevel + "\",\"2023 Donor\":\"" + donation + "\"}]}")

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

			// update aggregations
			err = db.UpdateAggregations(order)
			if err != nil {
				log.Errorf("error updating aggregations %v\n", err)
				w.WriteHeader((http.StatusInternalServerError))
				return
			}

			user, err := db.GetUser(order.UserName)
			if err != nil {
				log.Errorf("error getting user %v\n", err)
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
		/*
			if order.TotalTickets > 0 {
				ticketRevenue := order.Total - order.Donation
				tixSold, err := db.GetAggregation(fields.TotalTicketsSold)
				if err != nil {
					log.Errorf("error getting tickets sold agg: %v\n", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				tixSold.UpdateAggregation(tixSold.Quantity + order.TotalTickets, tixSold.Revenue + ticketRevenue)
				if err != nil {
					log.Errorf("error updating tickets sold agg: %v\n", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}



				cabinTixSold := order.AdultCabin + order.ChildCabin + order.ToddlerCabin
				tentTixSold := order.AdultTent + order.ChildTent + order.ToddlerTent
				satTixSold := order.AdultSat + order.ChildSat + order.ToddlerSat
				adultTixSold := order.AdultCabin + order.AdultTent + order.AdultSat
				childTixSold := order.ChildCabin + order.ChildTent + order.ChildSat
				toddlerTixSold := order.ToddlerCabin + order.ToddlerTent + order.ToddlerSat

				// remove at hard launch
				softLaunchSold, err := db.GetAggregation(fields.SoftLaunchSold)
				if err != nil {
					log.Errorf("error getting soft launch sales agg: %v\n", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				softLaunchSold.UpdateAggregation(softLaunchSold.Quantity + order.TotalTickets, softLaunchSold.Revenue + ticketRevenue)

				// end remove

				if cabinTixSold > 0 {
					cabinTixTotal, err := db.GetAggregation(fields.CabinSold)
					if err != nil {
						log.Errorf("error getting cabin tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					err = cabinTixTotal.UpdateAggregationFromOrder(order)
					if err != nil {
						log.Errorf("error updating cabin tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}

				if tentTixSold > 0 {
					tentTixTotal, err := db.GetAggregation(fields.TentSold)
					if err != nil {
						log.Errorf("error getting tent tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					err = tentTixTotal.UpdateAggregationFromOrder(order)
					if err != nil {
						log.Errorf("error updating tent tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}

				if satTixSold > 0 {
					satTixTotal, err := db.GetAggregation(fields.SatSold)
					if err != nil {
						log.Errorf("error getting sat tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					err = satTixTotal.UpdateAggregationFromOrder(order)
					if err != nil {
						log.Errorf("error updating sat tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}

				if adultTixSold > 0 {
					adultTixTotal, err := db.GetAggregation(fields.AdultSold)
					if err != nil {
						log.Errorf("error getting adult tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					err = adultTixTotal.UpdateAggregationFromOrder(order)
					if err != nil {
						log.Errorf("error updating adult tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}

				if childTixSold > 0 {
					childTixTotal, err := db.GetAggregation(fields.ChildSold)
					if err != nil {
						log.Errorf("error getting child tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					err = childTixTotal.UpdateAggregationFromOrder(order)
					if err != nil {
						log.Errorf("error updating child tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}

				if toddlerTixSold > 0 {
					toddlerTixTotal, err := db.GetAggregation(fields.ToddlerSold)
					if err != nil {
						log.Errorf("error getting toddler tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					err = toddlerTixTotal.UpdateAggregationFromOrder(order)
					if err != nil {
						log.Errorf("error updating toddler tickets sold agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}

				if order.Donation > 0 {
					donationRecv, err := db.GetAggregation(fields.DonationsRecv)
					if err != nil {
						log.Errorf("error getting donations agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					err = donationRecv.UpdateAggregationFromOrder(order)
					if err != nil {
						log.Errorf("error updating donations agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					scholarshipTix, err := db.GetAggregation(fields.ScholarshipTickets)
					if err != nil {
						log.Errorf("error getting scholarship tickets agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					err = scholarshipTix.UpdateAggregationFromOrder(order)
					if err != nil {
						log.Errorf("error updating scholarship tickets agg: %v\n", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}
			} */

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

		err = order.UpdateOrderStatus("processing")
		if err != nil {
			log.Errorf("error updating order payment status: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
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

	default:
		log.Errorf("Unhandled event type: %s\n", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}
