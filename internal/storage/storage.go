package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indicates that a listing could not be located in the backing store.
var ErrNotFound = errors.New("listing not found")

// ErrUserExists indicates that a user already exists with the given unique value.
var ErrUserExists = errors.New("user already exists")

// Listing represents the metadata and generated insights for a real estate listing.
type Listing struct {
	ID             string        `json:"id"`
	OwnerID        string        `json:"owner_id,omitempty"`
	Address        string        `json:"address"`
	Neighborhood   string        `json:"neighborhood,omitempty"`
	City           string        `json:"city,omitempty"`
	PropertyType   string        `json:"property_type,omitempty"`
	Condition      string        `json:"condition,omitempty"`
	Balcony        bool          `json:"balcony,omitempty"`
	Floor          string        `json:"floor,omitempty"`
	Association    string        `json:"association,omitempty"`
	Length         string        `json:"length,omitempty"`
	Tone           string        `json:"tone"`
	TargetAudience string        `json:"target_audience"`
	Highlights     []string      `json:"highlights"`
	ImageURL       string        `json:"image_url,omitempty"`
	Fee            int           `json:"fee,omitempty"`
	LivingArea     float64       `json:"living_area,omitempty"`
	Rooms          float64       `json:"rooms,omitempty"`
	Sections       []Section     `json:"sections,omitempty"`
	FullCopy       string        `json:"full_copy,omitempty"`
	History        History       `json:"section_history,omitempty"`
	Status         Status        `json:"status,omitempty"`
	Insights       Insights      `json:"insights,omitempty"`
	Details        Details       `json:"details,omitempty"`
	StyleProfile   *StyleProfile `json:"style_profile,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
}

// Section represents an editable block of text in the listing description.
type Section struct {
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Highlights []string `json:"highlights,omitempty"`
}

// Insights aggregates AI/automation derived metadata for a listing.
type Insights struct {
	Geodata GeodataInsights `json:"geodata,omitempty"`
	Vision  VisionInsights  `json:"vision,omitempty"`
}

// Status represents pipeline progress for the listing.
type Status struct {
	Data    string `json:"data"`
	Vision  string `json:"vision"`
	Geodata string `json:"geodata"`
	Text    string `json:"text"`
}

// VisionInsights stores AI-derived understanding of listing images.
type VisionInsights struct {
	Summary        string   `json:"summary"`
	RoomType       string   `json:"room_type"`
	Style          string   `json:"style"`
	NotableDetails []string `json:"notable_details"`
	ColorPalette   []string `json:"color_palette"`
	Tags           []string `json:"tags"`
}

// Details aggregates the richer structured data from the new form.
type Details struct {
	Meta        MetaInfo        `json:"meta"`
	Property    PropertyInfo    `json:"property"`
	Association AssociationInfo `json:"association"`
	Area        AreaInfo        `json:"area"`
	Advantages  []string        `json:"advantages"`
	Media       MediaLibrary    `json:"media"`
}

// MediaLibrary stores related assets (photos, renders, etc.).
type MediaLibrary struct {
	Images []ImageAsset `json:"images,omitempty"`
}

// ImageAsset represents a stored image in S3 or AI renders.
type ImageAsset struct {
	URL       string    `json:"url"`
	Key       string    `json:"key,omitempty"`
	Label     string    `json:"label,omitempty"`
	Source    string    `json:"source,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Cover     bool      `json:"cover,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// MetaInfo controls tone and text strategy.
type MetaInfo struct {
	DesiredWordCount int    `json:"desired_word_count"`
	Tone             string `json:"tone"`
	TargetAudience   string `json:"target_audience"`
	LanguageVariant  string `json:"language_variant"`
	StyleProfileID   string `json:"style_profile_id"`
}

// PropertyInfo stores property-level facts.
type PropertyInfo struct {
	Address             string  `json:"address"`
	PostalCode          string  `json:"postal_code"`
	City                string  `json:"city"`
	Area                string  `json:"area"`
	Municipality        string  `json:"municipality"`
	PropertyType        string  `json:"property_type"`
	Tenure              string  `json:"tenure"`
	Rooms               float64 `json:"rooms"`
	LivingArea          float64 `json:"living_area"`
	AdditionalArea      float64 `json:"additional_area"`
	Floor               string  `json:"floor"`
	NumberOfFloors      string  `json:"number_of_floors"`
	Elevator            bool    `json:"elevator"`
	YearBuilt           int     `json:"year_built"`
	YearRenovated       int     `json:"year_renovated"`
	Condition           string  `json:"condition"`
	PlanSummary         string  `json:"plan_summary"`
	InteriorStyle       string  `json:"interior_style"`
	Flooring            string  `json:"flooring"`
	CeilingHeight       string  `json:"ceiling_height"`
	LightIntake         string  `json:"light_intake"`
	KitchenDescription  string  `json:"kitchen_description"`
	BedroomDescription  string  `json:"bedroom_description"`
	LivingDescription   string  `json:"living_description"`
	BathroomDescription string  `json:"bathroom_description"`
	ExtraRooms          string  `json:"extra_rooms"`
	OutdoorDescription  string  `json:"outdoor_description"`
	StorageDescription  string  `json:"storage_description"`
	ParkingDescription  string  `json:"parking_description"`
	EnergyClass         string  `json:"energy_class"`
	Heating             string  `json:"heating"`
	FeePerMonth         int     `json:"fee_per_month"`
	OperatingCost       int     `json:"operating_cost"`
	ListPrice           int     `json:"list_price"`
	PriceText           string  `json:"price_text"`
}

// AssociationInfo describes the HOA or property association.
type AssociationInfo struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	FinancialSummary   string `json:"financial_summary"`
	DebtPerSquareMeter int    `json:"debt_per_square_meter"`
	CommonAreas        string `json:"common_areas"`
	RenovationsDone    string `json:"renovations_done"`
	RenovationsPlanned string `json:"renovations_planned"`
	AdditionalInfo     string `json:"additional_info"`
}

// AreaInfo contains surrounding neighborhood data.
type AreaInfo struct {
	Summary       string `json:"summary"`
	Transport     string `json:"transport"`
	Service       string `json:"service"`
	Schools       string `json:"schools"`
	NatureLeisure string `json:"nature_leisure"`
	Other         string `json:"other"`
}

// History maps section slugs to previous versions.
type History map[string][]SectionVersion

// SectionVersion tracks historical changes to a section.
type SectionVersion struct {
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	Source         string    `json:"source"`
	Instruction    string    `json:"instruction,omitempty"`
	Tone           string    `json:"tone,omitempty"`
	TargetAudience string    `json:"target_audience,omitempty"`
	Highlights     []string  `json:"highlights,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// GeodataInsights contains contextual information about the neighborhood.
type GeodataInsights struct {
	PointsOfInterest []PointOfInterest `json:"points_of_interest,omitempty"`
	Transit          []TransitInfo     `json:"transit,omitempty"`
}

// PointOfInterest represents a notable location near the property.
type PointOfInterest struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Distance string `json:"distance"`
}

// TransitInfo captures nearby transport options with estimated travel times.
type TransitInfo struct {
	Mode        string `json:"mode"`
	Description string `json:"description"`
}

// StyleProfile describes a stored tone-of-voice with sample texts.
type StyleProfile struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Tone           string     `json:"tone"`
	Guidelines     string     `json:"guidelines"`
	ExampleTexts   []string   `json:"example_texts"`
	ForbiddenWords []string   `json:"forbidden_words"`
	CustomModel    string     `json:"custom_model"`
	DatasetURI     string     `json:"dataset_uri"`
	LastTrainedAt  *time.Time `json:"last_trained_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// User represents an authenticated account that owns listings.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Store defines the persistence behaviors the application relies on.
type Store interface {
	CreateListing(ctx context.Context, input Listing) (Listing, error)
	ListListings(ctx context.Context) ([]Listing, error)
	ListListingsByOwner(ctx context.Context, ownerID string) ([]Listing, error)
	ListAllListings(ctx context.Context) ([]Listing, error)
	GetListing(ctx context.Context, id string) (Listing, error)
	UpdateListingSections(ctx context.Context, id string, sections []Section, fullCopy string, history History, status Status) (Listing, error)
	UpdateListingDetails(ctx context.Context, id string, details Details, imageURL string) (Listing, error)
	UpdateInsights(ctx context.Context, id string, insights Insights, status Status) (Listing, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
	DeleteListing(ctx context.Context, id string) error
	SaveStyleProfile(ctx context.Context, profile StyleProfile) (StyleProfile, error)
	ListStyleProfiles(ctx context.Context) ([]StyleProfile, error)
	GetStyleProfile(ctx context.Context, id string) (StyleProfile, error)
	CreateUser(ctx context.Context, user User) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	Close()
}

// NewStore selects a backing store based on whether a database URL is provided.
func NewStore(ctx context.Context, databaseURL string) (Store, error) {
	if databaseURL == "" {
		return NewInMemoryStore(), nil
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := ensureSchema(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &PostgresStore{pool: pool}, nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS listings (
        id TEXT PRIMARY KEY,
        owner_id TEXT,
        address TEXT NOT NULL,
        neighborhood TEXT,
        city TEXT,
        property_type TEXT,
        condition TEXT,
        balcony BOOLEAN,
        floor TEXT,
        association TEXT,
        length TEXT,
        tone TEXT NOT NULL,
        target_audience TEXT NOT NULL,
        highlights TEXT[],
        image_url TEXT,
        fee INTEGER,
        living_area DOUBLE PRECISION,
		rooms DOUBLE PRECISION,
		sections JSONB DEFAULT '[]'::jsonb,
        full_copy TEXT,
        section_history JSONB DEFAULT '{}'::jsonb,
        pipeline_status JSONB DEFAULT '{}'::jsonb,
        details JSONB DEFAULT '{}'::jsonb,
		insights JSONB DEFAULT '{}'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`)
	if err != nil {
		return fmt.Errorf("create listings table: %w", err)
	}

	var schemaAlters = []string{
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS neighborhood TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS owner_id TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS city TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS property_type TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS condition TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS balcony BOOLEAN`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS floor TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS association TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS length TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS image_url TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS fee INTEGER`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS living_area DOUBLE PRECISION`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS rooms DOUBLE PRECISION`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS sections JSONB DEFAULT '[]'::jsonb`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS full_copy TEXT`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS section_history JSONB DEFAULT '{}'::jsonb`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS pipeline_status JSONB DEFAULT '{}'::jsonb`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS details JSONB DEFAULT '{}'::jsonb`,
		`ALTER TABLE listings ADD COLUMN IF NOT EXISTS insights JSONB DEFAULT '{}'::jsonb`,
	}
	for _, stmt := range schemaAlters {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("alter listings table: %w", err)
		}
	}

	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS style_profiles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		tone TEXT,
		guidelines TEXT,
		example_texts TEXT[],
		forbidden_words TEXT[],
		custom_model TEXT,
		dataset_uri TEXT,
		last_trained_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create style_profiles table: %w", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE style_profiles ADD COLUMN IF NOT EXISTS custom_model TEXT`); err != nil {
		return fmt.Errorf("alter style_profiles custom_model: %w", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE style_profiles ADD COLUMN IF NOT EXISTS dataset_uri TEXT`); err != nil {
		return fmt.Errorf("alter style_profiles dataset_uri: %w", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE style_profiles ADD COLUMN IF NOT EXISTS last_trained_at TIMESTAMPTZ`); err != nil {
		return fmt.Errorf("alter style_profiles last_trained_at: %w", err)
	}

	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create users table: %w", err)
	}

	return nil
}
