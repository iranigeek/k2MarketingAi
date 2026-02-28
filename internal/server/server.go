package server

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"k2MarketingAi/internal/auth"
	"k2MarketingAi/internal/billing"
	"k2MarketingAi/internal/brfintel"
	"k2MarketingAi/internal/listings"
	"k2MarketingAi/internal/vision"
)

const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 8 * time.Minute
)

// New constructs the HTTP server with routes and middleware.
func New(port string, authHandler auth.Handler, authMiddleware auth.Middleware, usageLimiter auth.UsageLimiter, listingHandler listings.Handler, visionHandler vision.Handler, brfIntelHandler brfintel.Handler, billingHandler billing.Handler, staticFS http.Handler) *http.Server {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(authMiddleware.InjectUser)

	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	router.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/logout", authHandler.Logout)
			r.Get("/me", authHandler.Me)
		})

		// Stripe webhook – no auth required (verified via signature).
		r.Post("/billing/webhook", billingHandler.HandleWebhook)
		// Public billing config (publishable key + pricing table ID).
		r.Get("/billing/config", billingHandler.GetConfig)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth)
			r.Post("/uploads", listingHandler.UploadMedia)
			// Billing endpoints (authenticated, no usage limit).
			r.Route("/billing", func(r chi.Router) {
				r.Get("/subscription", billingHandler.GetSubscription)
				r.Post("/checkout", billingHandler.CreateCheckout)
				r.Post("/portal", billingHandler.CreatePortal)
			})
			r.Route("/listings", func(r chi.Router) {
				r.Get("/", listingHandler.List)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", listingHandler.Get)
					r.Post("/images", listingHandler.AttachImage)
					r.Post("/annual-report", listingHandler.AttachAnnualReport)
					r.Delete("/annual-report", listingHandler.DetachAnnualReport)
					r.Patch("/sections/{slug}", listingHandler.UpdateSection)
					r.Delete("/sections/{slug}", listingHandler.DeleteSection)
					r.Get("/export", listingHandler.ExportFullCopy)
					r.Delete("/", listingHandler.DeleteListing)
				})
			})
			r.Route("/style-profiles", func(r chi.Router) {
				r.Get("/", listingHandler.ListStyleProfiles)
				r.Post("/", listingHandler.SaveStyleProfile)
			})
			r.Route("/templates", func(r chi.Router) {
				r.Get("/", listingHandler.ListTemplates)
				r.Post("/", listingHandler.SaveTemplate)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", listingHandler.GetTemplate)
					r.Delete("/", listingHandler.DeleteTemplate)
				})
			})
			r.Get("/events", listingHandler.StreamEvents)
			r.Route("/brf-intel", func(r chi.Router) {
				r.Get("/reports", brfIntelHandler.List)
				r.Get("/recent", brfIntelHandler.RecentReports)
				r.Get("/reports/{id}", brfIntelHandler.Get)
				r.Delete("/reports/{id}", brfIntelHandler.Delete)
			})

			// ── AI-powered routes: require paid subscription or free-tier quota ──
			r.Group(func(r chi.Router) {
				r.Use(usageLimiter.RequirePaidOrQuota)
				r.Post("/listings", listingHandler.Create)
				r.Post("/listings/{id}/sections/{slug}/rewrite", listingHandler.RewriteSection)
				r.Post("/annual-reports/extract", listingHandler.ExtractAnnualReport)
				r.Post("/annual-reports/summarize", listingHandler.SummarizeAnnualReport)
				r.Route("/vision", func(r chi.Router) {
					r.Post("/analyze", visionHandler.Analyze)
					r.Post("/design", visionHandler.Design)
					r.Post("/render", visionHandler.Render)
				})
				r.Post("/brf-intel/analyze", brfIntelHandler.Analyze)
				r.Post("/brf-intel/analyze-pdf", brfIntelHandler.AnalyzePDF)
				r.Post("/brf-intel/analyze-listing/{id}", brfIntelHandler.AnalyzeFromListing)
				r.Post("/brf-intel/score-quick", brfIntelHandler.ScoreQuick)
			})
		})
	})

	// Serve the static frontend
	router.Handle("/*", staticFS)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("server ready on", srv.Addr)
	return srv
}
