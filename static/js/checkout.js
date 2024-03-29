// This is a public sample test API key.
// Don’t submit any personally identifiable information in requests made with this key.
// Sign in to see your own test API key embedded in code samples.
const pk =
  window.location.hostname === "127.0.0.1.nip.io"
    ? "pk_test_TYooMQauvdEDq54NiTphI7jx"
    : "pk_live_51K3PO6IjvlmyJAlxhV2DLqZyChqriEDWkpw4GpIIT5BtowCdoCzbwVylA4pBYtPdI1EeZIvFM71J1y9ECLcNExTy00LKDowq6n";
const stripe = Stripe(pk);

let elements;

let items = [];
let username = "";
let usertype = "";
let paymentIntentId = "";
const ticketCart = document.querySelector("#ticket-cart");
if (ticketCart.hasAttribute("cartData")) {
  items = JSON.parse(ticketCart.getAttribute("cartData")).items;
  username = ticketCart.getAttribute("username");
  usertype = ticketCart.getAttribute("usertype");
  ordertype = ticketCart.getAttribute("ordertype");
}

initialize();
checkStatus();

document
  .querySelector("#payment-form")
  .addEventListener("submit", handleSubmit);

// how do i get items
// pass into template from go & call func in html js script tag
// get from query params within this js script
// set a hidden html element data attr to it & get that via queryselector in here
// mostly thinking about whether the handleSubmit bind above captures element before
// init is called if doing first one

// Fetches a payment intent and captures the client secret
async function initialize() {
  const currSecret = new URLSearchParams(window.location.search).get(
    "payment_intent_client_secret"
  );

  if (currSecret) {
    return;
  }

  setLoading(true);
  const response = await fetch("/create-payment-intent", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ items, username, usertype, ordertype }),
  });
  const { clientSecret, total, intentId } = await response.json();
  paymentIntentId = intentId;

  const appearance = {
    theme: "stripe",
  };
  elements = stripe.elements({ appearance, clientSecret });

  document.querySelector("#order-total").value = `$${total.toFixed(2)}`;
  document.querySelector("#total-div").classList.remove("hidden");
  const paymentElement = elements.create("payment");
  paymentElement.mount("#payment-element");
  setLoading(false);
}

async function handleSubmit(e) {
  e.preventDefault();
  setLoading(true);

  const { error } = await stripe.confirmPayment({
    elements,
    confirmParams: {
      return_url:
        (window.location.host.startsWith("127") ? "http://" : "https://") +
        window.location.host +
        "/checkout-complete?payment_id=" +
        paymentIntentId,
    },
  });

  if (error.type === "card_error" || error.type === "validation_error") {
    showMessage(error.message);
  } else {
    showMessage("An unexpected error occured.");
  }

  setLoading(false);
}

// Fetches the payment intent status after payment submission
async function checkStatus() {
  const clientSecret = new URLSearchParams(window.location.search).get(
    "payment_intent_client_secret"
  );

  if (!clientSecret) {
    return;
  }

  setLoading(true);

  const { paymentIntent } = await stripe.retrievePaymentIntent(clientSecret);

  switch (paymentIntent.status) {
    case "succeeded":
      showMessage("Payment succeeded!");
      window.location.replace(
        "/checkout-complete?payment_id=" + paymentIntentId
      );
      break;
    case "processing":
      showMessage("Your payment is processing.");
      break;
    case "requires_payment_method":
      showMessage("Your payment was not successful, please try again.");
      break;
    default:
      showMessage("Something went wrong.");
      break;
  }
}

// ------- UI helpers -------

function showMessage(messageText) {
  setLoading(false);
  document.querySelector("#submit").disabled = true;
  document.querySelector("#submit").classList.add("hidden");
  document.querySelector("#button-text").classList.add("hidden");

  const messageContainer = document.querySelector("#payment-message");

  messageContainer.classList.remove("hidden");
  messageContainer.textContent = messageText;

  setTimeout(function () {
    messageContainer.classList.add("hidden");
    messageText.textContent = "";
  }, 7500);
}

// Show a spinner on payment submission
function setLoading(isLoading) {
  if (isLoading) {
    // Disable the button and show a spinner
    document.querySelector("#submit").disabled = true;
    document.querySelector("#spinner").classList.remove("hidden");
    document.querySelector("#button-text").classList.add("hidden");
  } else {
    document.querySelector("#submit").disabled = false;
    document.querySelector("#spinner").classList.add("hidden");
    document.querySelector("#button-text").classList.remove("hidden");
  }
}
