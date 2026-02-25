package billing

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/stripe/stripe-go/v82"
	billingportalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"

	"k2MarketingAi/internal/auth"
	"k2MarketingAi/internal/storage"
)

// Config holds Stripe-specific configuration.
type Config struct {
	SecretKey      string
	WebhookSecret  string
	PriceID        string
	SuccessURL     string
	CancelURL      string
	PublishableKey string
	PricingTableID string
}

// Handler exposes billing-related HTTP endpoints.
type Handler struct {
	Store  storage.Store
	Config Config
}

// NewHandler creates a billing handler and initialises the Stripe SDK.
func NewHandler(store storage.Store, cfg Config) Handler {
	stripe.Key = cfg.SecretKey
	return Handler{Store: store, Config: cfg}
}

// --- Helper ---

func jsonResp(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// --- Endpoints ---

// GetConfig returns public Stripe config (publishable key + pricing table ID).
// GET /api/billing/config
func (h Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, http.StatusOK, map[string]string{
		"publishable_key":  h.Config.PublishableKey,
		"pricing_table_id": h.Config.PricingTableID,
	})
}

// GetSubscription returns the current user's subscription status.
// GET /api/billing/subscription
func (h Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	jsonResp(w, http.StatusOK, map[string]any{
		"subscription_status": user.SubscriptionStatus,
		"subscription_id":     user.SubscriptionID,
		"plan_id":             user.PlanID,
		"stripe_customer_id":  user.StripeCustomerID,
		"usage_count":         user.UsageCount,
		"usage_limit":         storage.FreeUsageLimit,
	})
}

// CreateCheckout creates a Stripe Checkout Session for a subscription.
// POST /api/billing/checkout
func (h Handler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.Config.PriceID == "" {
		http.Error(w, "pris ej konfigurerat – kontakta support", http.StatusServiceUnavailable)
		return
	}

	// Ensure the user has a Stripe Customer.
	customerID := user.StripeCustomerID
	if customerID == "" {
		params := &stripe.CustomerParams{
			Email: stripe.String(user.Email),
			Params: stripe.Params{
				Metadata: map[string]string{
					"user_id": user.ID,
				},
			},
		}
		c, err := customer.New(params)
		if err != nil {
			log.Printf("stripe: create customer: %v", err)
			http.Error(w, "kunde inte skapa kundprofil", http.StatusInternalServerError)
			return
		}
		customerID = c.ID
		if err := h.Store.UpdateUserStripe(r.Context(), user.ID, customerID, user.SubscriptionID, user.SubscriptionStatus, user.PlanID); err != nil {
			log.Printf("store: save stripe customer id: %v", err)
		}
	}

	// Optional: read a price_id override from the body.
	priceID := h.Config.PriceID
	var body struct {
		PriceID string `json:"price_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.PriceID != "" {
		priceID = body.PriceID
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(h.Config.SuccessURL),
		CancelURL:  stripe.String(h.Config.CancelURL),
	}

	sess, err := checkoutsession.New(params)
	if err != nil {
		log.Printf("stripe: create checkout session: %v", err)
		http.Error(w, "kunde inte skapa betalningssession", http.StatusInternalServerError)
		return
	}

	jsonResp(w, http.StatusOK, map[string]string{
		"url": sess.URL,
	})
}

// CreatePortal creates a Stripe Customer Portal session so the user can manage billing.
// POST /api/billing/portal
func (h Handler) CreatePortal(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if user.StripeCustomerID == "" {
		http.Error(w, "ingen aktiv prenumeration", http.StatusBadRequest)
		return
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(user.StripeCustomerID),
		ReturnURL: stripe.String(h.Config.SuccessURL),
	}
	sess, err := billingportalsession.New(params)
	if err != nil {
		log.Printf("stripe: create portal session: %v", err)
		http.Error(w, "kunde inte öppna kundportalen", http.StatusInternalServerError)
		return
	}

	jsonResp(w, http.StatusOK, map[string]string{
		"url": sess.URL,
	})
}

// HandleWebhook processes incoming Stripe webhook events.
// POST /api/billing/webhook
func (h Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBodySize = 65536
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var event stripe.Event
	if h.Config.WebhookSecret != "" {
		event, err = webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), h.Config.WebhookSecret)
		if err != nil {
			log.Printf("stripe webhook: signature verification failed: %v", err)
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
	} else {
		// No webhook secret configured – parse raw (dev mode).
		if err := json.Unmarshal(body, &event); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
	}

	log.Printf("stripe webhook: %s", event.Type)

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(r, event)
	case "customer.subscription.updated":
		h.handleSubscriptionUpdated(r, event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(r, event)
	case "invoice.payment_failed":
		h.handlePaymentFailed(r, event)
	default:
		log.Printf("stripe webhook: unhandled event type %s", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}

// --- Webhook event handlers ---

func (h Handler) handleCheckoutCompleted(r *http.Request, event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		log.Printf("stripe webhook: unmarshal checkout session: %v", err)
		return
	}

	customerID := ""
	if sess.Customer != nil {
		customerID = sess.Customer.ID
	}
	subscriptionID := ""
	if sess.Subscription != nil {
		subscriptionID = sess.Subscription.ID
	}

	// Try to find the user – first by Stripe customer, then by client_reference_id
	// (used by the Stripe Pricing Table embed).
	var user *storage.User

	if customerID != "" {
		u, err := h.Store.GetUserByStripeCustomer(r.Context(), customerID)
		if err == nil {
			user = &u
		}
	}
	if user == nil && sess.ClientReferenceID != "" {
		u, err := h.Store.GetUserByID(r.Context(), sess.ClientReferenceID)
		if err != nil {
			log.Printf("stripe webhook: find user by client_reference_id %s: %v", sess.ClientReferenceID, err)
			return
		}
		user = &u
	}
	if user == nil {
		log.Printf("stripe webhook: could not resolve user (customer=%s, ref=%s)", customerID, sess.ClientReferenceID)
		return
	}

	planID := h.Config.PriceID
	if planID == "" || planID == "200" {
		planID = "pricing-table"
	}

	if err := h.Store.UpdateUserStripe(r.Context(), user.ID, customerID, subscriptionID, "active", planID); err != nil {
		log.Printf("stripe webhook: update user stripe: %v", err)
		return
	}

	// Auto-approve user upon successful subscription.
	if !user.Approved {
		if err := h.Store.ApproveUser(r.Context(), user.ID, true); err != nil {
			log.Printf("stripe webhook: approve user: %v", err)
		}
	}

	log.Printf("stripe: checkout completed for user %s (customer %s)", user.ID, customerID)
}

func (h Handler) handleSubscriptionUpdated(r *http.Request, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		log.Printf("stripe webhook: unmarshal subscription: %v", err)
		return
	}

	customerID := sub.Customer.ID
	user, err := h.Store.GetUserByStripeCustomer(r.Context(), customerID)
	if err != nil {
		log.Printf("stripe webhook: find user for customer %s: %v", customerID, err)
		return
	}

	status := string(sub.Status)
	planID := ""
	if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
		planID = sub.Items.Data[0].Price.ID
	}

	if err := h.Store.UpdateUserStripe(r.Context(), user.ID, customerID, sub.ID, status, planID); err != nil {
		log.Printf("stripe webhook: update subscription: %v", err)
	}

	log.Printf("stripe: subscription %s updated to %s for user %s", sub.ID, status, user.ID)
}

func (h Handler) handleSubscriptionDeleted(r *http.Request, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		log.Printf("stripe webhook: unmarshal subscription: %v", err)
		return
	}

	customerID := sub.Customer.ID
	user, err := h.Store.GetUserByStripeCustomer(r.Context(), customerID)
	if err != nil {
		log.Printf("stripe webhook: find user for customer %s: %v", customerID, err)
		return
	}

	if err := h.Store.UpdateUserStripe(r.Context(), user.ID, customerID, "", "canceled", ""); err != nil {
		log.Printf("stripe webhook: cancel subscription: %v", err)
	}

	log.Printf("stripe: subscription canceled for user %s", user.ID)
}

func (h Handler) handlePaymentFailed(r *http.Request, event stripe.Event) {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		log.Printf("stripe webhook: unmarshal invoice: %v", err)
		return
	}

	customerID := ""
	if inv.Customer != nil {
		customerID = inv.Customer.ID
	}
	if customerID == "" {
		return
	}

	user, err := h.Store.GetUserByStripeCustomer(r.Context(), customerID)
	if err != nil {
		log.Printf("stripe webhook: find user for customer %s: %v", customerID, err)
		return
	}

	if err := h.Store.UpdateUserStripe(r.Context(), user.ID, customerID, user.SubscriptionID, "past_due", user.PlanID); err != nil {
		log.Printf("stripe webhook: mark past_due: %v", err)
	}

	log.Printf("stripe: payment failed for user %s", user.ID)
	_ = fmt.Sprintf("Payment failed for user %s", user.Email) // placeholder for email notification
}
