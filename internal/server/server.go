package server

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"k2MarketingAi/internal/listings"
)

// New constructs the HTTP server with routes and middleware.
func New(port string, listingHandler listings.Handler, staticFS http.Handler) *http.Server {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	router.Route("/api", func(r chi.Router) {
		r.Route("/listings", func(r chi.Router) {
			r.Get("/", listingHandler.List)
			r.Post("/", listingHandler.Create)
		})
	})

	// Serve the static frontend
	router.Handle("/*", staticFS)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("server ready on", srv.Addr)
	return srv
}
