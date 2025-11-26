package flights

import (
	"fmt"
	"strings"
	"time"

	"github.com/dvitali/flights-mcp/internal/airports"
	"github.com/dvitali/flights-mcp/pkg/models"
)

// Validator validates flight search requests.
type Validator struct {
	airportDB *airports.Database
}

// NewValidator creates a new validator.
func NewValidator(db *airports.Database) *Validator {
	return &Validator{airportDB: db}
}

// ValidationResult contains the result of validation.
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// Validate validates a flight search request.
func (v *Validator) Validate(req *models.FlightSearchRequest) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Validate airport codes
	fromCode, err := airports.SanitizeAirportCode(req.FromAirport)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid origin airport code: %s", err))
	}

	toCode, err := airports.SanitizeAirportCode(req.ToAirport)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid destination airport code: %s", err))
	}

	// Check if same airport
	if fromCode != "" && toCode != "" && fromCode == toCode {
		result.Valid = false
		result.Errors = append(result.Errors, "Origin and destination airports cannot be the same")
	}

	// Validate departure date
	depDate, err := time.ParseInLocation("2006-01-02", req.DepartureDate, time.Local)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid departure date format: %s (use YYYY-MM-DD)", req.DepartureDate))
	} else {
		// Get today's date at midnight in local timezone
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		if depDate.Before(today) {
			result.Valid = false
			result.Errors = append(result.Errors, "Departure date cannot be in the past")
		}
	}

	// Validate return date if round-trip
	if req.TripType == models.TripTypeRoundTrip {
		if req.ReturnDate == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "Return date is required for round-trip flights")
		} else {
			retDate, err := time.ParseInLocation("2006-01-02", req.ReturnDate, time.Local)
			if err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("Invalid return date format: %s (use YYYY-MM-DD)", req.ReturnDate))
			} else if depDate.IsZero() == false && retDate.Before(depDate) {
				result.Valid = false
				result.Errors = append(result.Errors, "Return date cannot be before departure date")
			} else if depDate.IsZero() == false && retDate.Equal(depDate) {
				result.Warnings = append(result.Warnings, "Return date is the same as departure date - consider a one-way trip")
			}
		}
	}

	// Validate trip type
	if !models.IsValidTripType(req.TripType) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid trip type: %s (use 'one-way' or 'round-trip')", req.TripType))
	}

	// Validate seat class
	if !models.IsValidSeatClass(req.SeatClass) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid seat class: %s (use 'economy', 'premium-economy', 'business', or 'first')", req.SeatClass))
	}

	// Validate passengers
	if req.PassengersAdults < 1 {
		result.Valid = false
		result.Errors = append(result.Errors, "At least one adult passenger is required")
	}

	totalPassengers := req.PassengersAdults + req.PassengersChildren + req.PassengersInfantsSeat
	if totalPassengers > 9 {
		result.Valid = false
		result.Errors = append(result.Errors, "Maximum 9 passengers allowed per booking")
	}

	if req.PassengersInfantsLap > req.PassengersAdults {
		result.Valid = false
		result.Errors = append(result.Errors, "Number of lap infants cannot exceed number of adults")
	}

	// Check if airports are in database (warnings only)
	if v.airportDB != nil && fromCode != "" {
		if !v.airportDB.Exists(fromCode) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Origin airport '%s' not in our database", fromCode))
		}
	}

	if v.airportDB != nil && toCode != "" {
		if !v.airportDB.Exists(toCode) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Destination airport '%s' not in our database", toCode))
		}
	}

	return result
}

// ValidateAirportCode validates and sanitizes an airport code.
func (v *Validator) ValidateAirportCode(code string) (string, error) {
	return airports.SanitizeAirportCode(code)
}

// FormatResult formats the validation result as a string.
func (r *ValidationResult) FormatResult() string {
	var sb strings.Builder

	if !r.Valid {
		sb.WriteString("**Validation Failed:**\n")
		for _, err := range r.Errors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
		return sb.String()
	}

	if len(r.Warnings) > 0 {
		sb.WriteString("**Validation Passed with Warnings:**\n")
		for _, warn := range r.Warnings {
			sb.WriteString(fmt.Sprintf("- %s\n", warn))
		}
		sb.WriteString("\nParameters are valid for flight search.")
		return sb.String()
	}

	return "**Validation Passed:** All parameters are valid for flight search."
}
