package geodata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"k2MarketingAi/internal/storage"
)

const (
	poiRadiusMeters     = 1200
	transitRadiusMeters = 1500
	maxPOIResults       = 8
	maxTransitResults   = 4
)

// Config encapsulates the external API configuration.
type Config struct {
	GooglePlacesAPIKey string
	TrafficAPIKey      string
	CacheTTL           time.Duration
}

// Provider fetches contextual geodata for an address.
type Provider interface {
	Fetch(ctx context.Context, address string) (Summary, error)
}

// Summary contains the structured neighborhood context.
type Summary struct {
	PointsOfInterest []PointOfInterestSummary
	Transit          []TransitSummary
}

// PointOfInterestSummary is a lightweight POI representation.
type PointOfInterestSummary struct {
	Name     string
	Category string
	Distance string
}

// TransitSummary wraps information about transport options.
type TransitSummary struct {
	Mode        string
	Description string
}

// NewProvider wires a provider implementation based on the config.
func NewProvider(cfg Config) Provider {
	var base Provider
	if cfg.GooglePlacesAPIKey == "" {
		base = &staticProvider{}
	} else {
		base = &googleProvider{
			apiKey: cfg.GooglePlacesAPIKey,
			client: &http.Client{Timeout: 6 * time.Second},
		}
	}

	return wrapWithCache(base, cfg.CacheTTL)
}

func wrapWithCache(base Provider, ttl time.Duration) Provider {
	if ttl <= 0 {
		return base
	}

	return &cachedProvider{
		base:    base,
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

type cachedProvider struct {
	base    Provider
	ttl     time.Duration
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	summary Summary
	expires time.Time
}

func (c *cachedProvider) Fetch(ctx context.Context, address string) (Summary, error) {
	key := normalizeAddress(address)
	now := time.Now()

	c.mu.RLock()
	if entry, ok := c.entries[key]; ok && entry.expires.After(now) {
		c.mu.RUnlock()
		return entry.summary, nil
	}
	c.mu.RUnlock()

	summary, err := c.base.Fetch(ctx, address)
	if err != nil {
		return Summary{}, err
	}

	c.mu.Lock()
	c.entries[key] = cacheEntry{
		summary: summary,
		expires: now.Add(c.ttl),
	}
	c.mu.Unlock()

	return summary, nil
}

func normalizeAddress(address string) string {
	trimmed := strings.TrimSpace(strings.ToLower(address))
	parts := strings.Fields(trimmed)
	return strings.Join(parts, " ")
}

type googleProvider struct {
	apiKey string
	client *http.Client
}

func (p *googleProvider) Fetch(ctx context.Context, address string) (Summary, error) {
	coords, err := p.geocode(ctx, address)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{}

	if pois, err := p.searchPlaces(ctx, coords, "point_of_interest", poiRadiusMeters, maxPOIResults); err == nil {
		summary.PointsOfInterest = pois
	} else {
		log.Printf("geodata: poi search failed: %v", err)
	}

	if transit, err := p.searchTransit(ctx, coords, transitRadiusMeters, maxTransitResults); err == nil {
		summary.Transit = transit
	} else {
		log.Printf("geodata: transit search failed: %v", err)
	}

	return summary, nil
}

type latLng struct {
	Lat float64
	Lng float64
}

func (p *googleProvider) geocode(ctx context.Context, address string) (latLng, error) {
	params := url.Values{
		"address": []string{address},
		"key":     []string{p.apiKey},
	}

	var resp geocodeResponse
	if err := p.get(ctx, "https://maps.googleapis.com/maps/api/geocode/json", params, &resp); err != nil {
		return latLng{}, err
	}
	if resp.Status != "OK" || len(resp.Results) == 0 {
		return latLng{}, fmt.Errorf("geocode status %s", resp.Status)
	}

	loc := resp.Results[0].Geometry.Location
	return latLng{Lat: loc.Lat, Lng: loc.Lng}, nil
}

func (p *googleProvider) searchPlaces(ctx context.Context, point latLng, placeType string, radius, limit int) ([]PointOfInterestSummary, error) {
	params := url.Values{
		"location": []string{fmt.Sprintf("%f,%f", point.Lat, point.Lng)},
		"radius":   []string{fmt.Sprintf("%d", radius)},
		"type":     []string{placeType},
		"key":      []string{p.apiKey},
	}

	var resp placesResponse
	if err := p.get(ctx, "https://maps.googleapis.com/maps/api/place/nearbysearch/json", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		if resp.Status == "ZERO_RESULTS" {
			return []PointOfInterestSummary{}, nil
		}
		return nil, fmt.Errorf("places status %s", resp.Status)
	}

	pois := make([]PointOfInterestSummary, 0, len(resp.Results))
	for _, result := range resp.Results {
		category := mapGoogleCategory(result.Types)
		distance := formatDistanceMeters(distanceMeters(point, latLng{Lat: result.Geometry.Location.Lat, Lng: result.Geometry.Location.Lng}))
		name := result.Name
		if result.Vicinity != "" {
			name = fmt.Sprintf("%s – %s", result.Name, result.Vicinity)
		}
		pois = append(pois, PointOfInterestSummary{
			Name:     name,
			Category: category,
			Distance: distance,
		})
		if len(pois) >= limit {
			break
		}
	}

	return pois, nil
}

func (p *googleProvider) searchTransit(ctx context.Context, point latLng, radius, limit int) ([]TransitSummary, error) {
	params := url.Values{
		"location": []string{fmt.Sprintf("%f,%f", point.Lat, point.Lng)},
		"radius":   []string{fmt.Sprintf("%d", radius)},
		"type":     []string{"transit_station"},
		"key":      []string{p.apiKey},
	}

	var resp placesResponse
	if err := p.get(ctx, "https://maps.googleapis.com/maps/api/place/nearbysearch/json", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		if resp.Status == "ZERO_RESULTS" {
			return []TransitSummary{}, nil
		}
		return nil, fmt.Errorf("places status %s", resp.Status)
	}

	transit := make([]TransitSummary, 0, len(resp.Results))
	for _, result := range resp.Results {
		mode := mapTransitMode(result.Types)
		desc := result.Vicinity
		if desc == "" {
			desc = result.Name
		} else {
			desc = fmt.Sprintf("%s – %s", result.Name, desc)
		}
		transit = append(transit, TransitSummary{
			Mode:        mode,
			Description: desc,
		})
		if len(transit) >= limit {
			break
		}
	}

	return transit, nil
}

func (p *googleProvider) get(ctx context.Context, baseURL string, params url.Values, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = params.Encode()

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google api status %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}

type geocodeResponse struct {
	Status  string `json:"status"`
	Results []struct {
		Geometry struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}

type placesResponse struct {
	Status  string        `json:"status"`
	Results []placeResult `json:"results"`
}

type placeResult struct {
	Name     string   `json:"name"`
	Vicinity string   `json:"vicinity"`
	Types    []string `json:"types"`
	Geometry struct {
		Location struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"location"`
	} `json:"geometry"`
}

func mapGoogleCategory(types []string) string {
	categories := []struct {
		Type string
		Name string
	}{
		{"cafe", "Caf?"},
		{"restaurant", "Restaurang"},
		{"park", "Park"},
		{"gym", "Gym"},
		{"supermarket", "Matbutik"},
		{"grocery_or_supermarket", "Matbutik"},
		{"shopping_mall", "Butik"},
		{"store", "Butik"},
		{"pharmacy", "Apotek"},
		{"hospital", "Sjukhus"},
		{"gas_station", "Bensinstation"},
		{"parking", "Parkering"},
		{"school", "Skola"},
		{"primary_school", "Skola"},
		{"secondary_school", "Skola"},
	}

	for _, mapping := range categories {
		for _, t := range types {
			if t == mapping.Type {
				return mapping.Name
			}
		}
	}
	return "Plats"
}

func mapTransitMode(types []string) string {
	modeMap := map[string]string{
		"subway_station":     "Tunnelbana",
		"train_station":      "Tåg",
		"light_rail_station": "Spårbunden",
		"bus_station":        "Buss",
		"transit_station":    "Transit",
	}

	for _, t := range types {
		if name, ok := modeMap[t]; ok {
			return name
		}
	}

	return "Transit"
}

func distanceMeters(a, b latLng) float64 {
	const earthRadius = 6371000 // meters
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180

	sinLat := math.Sin(dLat / 2)
	sinLng := math.Sin(dLng / 2)

	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLng*sinLng
	return 2 * earthRadius * math.Asin(math.Sqrt(h))
}

func formatDistanceMeters(m float64) string {
	if m < 1000 {
		return fmt.Sprintf("%dm", int(m+0.5))
	}
	return fmt.Sprintf("%.1f km", m/1000)
}

type staticProvider struct{}

var samplePOIs = [][]string{
	{"Björk Café & Bar", "Café", "250 m"},
	{"Stadshusparken", "Park", "450 m"},
	{"Balance Gym", "Gym", "600 m"},
	{"Matboden", "Matbutik", "200 m"},
}

var sampleTransit = [][]string{
	{"Tunnelbana", "3 minuter till stationen, 12 minuter till T-Centralen"},
	{"Buss", "Linje 52 till city var 6:e minut i rusningstid"},
	{"Pendeltåg", "8 minuter till stationen, 15 minuter till centralen"},
}

func (staticProvider) Fetch(_ context.Context, address string) (Summary, error) {
	rand.Seed(time.Now().UnixNano())
	addrPrefix := strings.Split(address, " ")[0]

	pois := make([]PointOfInterestSummary, 0, len(samplePOIs))
	for _, p := range samplePOIs {
		pois = append(pois, PointOfInterestSummary{
			Name:     fmt.Sprintf("%s %s", addrPrefix, p[0]),
			Category: p[1],
			Distance: p[2],
		})
	}

	transit := make([]TransitSummary, 0, len(sampleTransit))
	for _, t := range sampleTransit {
		transit = append(transit, TransitSummary{Mode: t[0], Description: t[1]})
	}

	return Summary{
		PointsOfInterest: pois,
		Transit:          transit,
	}, nil
}

// ToStorageInsights converts a Summary into the storage representation.
func ToStorageInsights(summary Summary) storage.GeodataInsights {
	pois := make([]storage.PointOfInterest, 0, len(summary.PointsOfInterest))
	for _, p := range summary.PointsOfInterest {
		pois = append(pois, storage.PointOfInterest{
			Name:     p.Name,
			Category: p.Category,
			Distance: p.Distance,
		})
	}

	transit := make([]storage.TransitInfo, 0, len(summary.Transit))
	for _, t := range summary.Transit {
		transit = append(transit, storage.TransitInfo{
			Mode:        t.Mode,
			Description: t.Description,
		})
	}

	return storage.GeodataInsights{
		PointsOfInterest: pois,
		Transit:          transit,
	}
}
