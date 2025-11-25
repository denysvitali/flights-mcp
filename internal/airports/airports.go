// Package airports provides airport data and lookup functionality.
package airports

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/dvitali/flights-mcp/pkg/models"
)

// airportsData represents the JSON structure for airports.
type airportsData struct {
	Airports []models.AirportInfo `json:"airports"`
}

// Database provides airport lookup functionality.
type Database struct {
	airports map[string]*models.AirportInfo
	mu       sync.RWMutex
}

// airportCodeRegex validates IATA airport codes (3 uppercase letters).
var airportCodeRegex = regexp.MustCompile(`^[A-Z]{3}$`)

// NewDatabase creates a new airport database.
func NewDatabase() *Database {
	return &Database{
		airports: make(map[string]*models.AirportInfo),
	}
}

// LoadFromFile loads airports from a JSON file.
func (db *Database) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read airports file: %w", err)
	}

	return db.LoadFromJSON(data)
}

// LoadFromJSON loads airports from JSON data.
func (db *Database) LoadFromJSON(data []byte) error {
	var airports airportsData
	if err := json.Unmarshal(data, &airports); err != nil {
		return fmt.Errorf("failed to parse airports JSON: %w", err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	db.airports = make(map[string]*models.AirportInfo, len(airports.Airports))
	for i := range airports.Airports {
		airport := &airports.Airports[i]
		airport.Code = strings.ToUpper(airport.Code)
		db.airports[airport.Code] = airport
	}

	return nil
}

// Get returns airport information by code.
func (db *Database) Get(code string) (*models.AirportInfo, bool) {
	sanitized, err := SanitizeAirportCode(code)
	if err != nil {
		return nil, false
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	airport, ok := db.airports[sanitized]
	return airport, ok
}

// Exists checks if an airport code exists in the database.
func (db *Database) Exists(code string) bool {
	_, ok := db.Get(code)
	return ok
}

// Count returns the number of airports in the database.
func (db *Database) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.airports)
}

// All returns all airports in the database.
func (db *Database) All() []*models.AirportInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()

	result := make([]*models.AirportInfo, 0, len(db.airports))
	for _, airport := range db.airports {
		result = append(result, airport)
	}
	return result
}

// SanitizeAirportCode sanitizes and validates an airport code.
func SanitizeAirportCode(code string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("airport code cannot be empty")
	}

	// Remove whitespace and convert to uppercase
	sanitized := strings.ToUpper(strings.TrimSpace(code))

	// Remove any non-letter characters
	sanitized = regexp.MustCompile(`[^A-Z]`).ReplaceAllString(sanitized, "")

	// Validate format
	if !airportCodeRegex.MatchString(sanitized) {
		return "", fmt.Errorf("invalid airport code format: '%s' (must be exactly 3 letters)", code)
	}

	return sanitized, nil
}

// IsValidAirportCode checks if a code is a valid IATA airport code format.
func IsValidAirportCode(code string) bool {
	_, err := SanitizeAirportCode(code)
	return err == nil
}
