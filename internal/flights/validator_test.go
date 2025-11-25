package flights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dvitali/flights-mcp/internal/airports"
	"github.com/dvitali/flights-mcp/pkg/models"
)

func setupValidator() *Validator {
	db := airports.NewDatabase()
	db.LoadFromJSON([]byte(`{
		"airports": [
			{"code": "JFK", "name": "JFK", "city": "NYC", "country": "USA"},
			{"code": "LAX", "name": "LAX", "city": "LA", "country": "USA"}
		]
	}`))
	return NewValidator(db)
}

func TestValidator_Validate_ValidRequest(t *testing.T) {
	v := setupValidator()

	futureDate := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	req := &models.FlightSearchRequest{
		FromAirport:      "JFK",
		ToAirport:        "LAX",
		DepartureDate:    futureDate,
		TripType:         models.TripTypeOneWay,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	result := v.Validate(req)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidator_Validate_InvalidAirportCode(t *testing.T) {
	v := setupValidator()

	req := &models.FlightSearchRequest{
		FromAirport:      "XX",
		ToAirport:        "LAX",
		DepartureDate:    "2025-12-15",
		TripType:         models.TripTypeOneWay,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	result := v.Validate(req)
	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "Invalid origin airport code")
}

func TestValidator_Validate_SameAirport(t *testing.T) {
	v := setupValidator()

	req := &models.FlightSearchRequest{
		FromAirport:      "JFK",
		ToAirport:        "JFK",
		DepartureDate:    "2025-12-15",
		TripType:         models.TripTypeOneWay,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	result := v.Validate(req)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "same")
}

func TestValidator_Validate_PastDate(t *testing.T) {
	v := setupValidator()

	pastDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	req := &models.FlightSearchRequest{
		FromAirport:      "JFK",
		ToAirport:        "LAX",
		DepartureDate:    pastDate,
		TripType:         models.TripTypeOneWay,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	result := v.Validate(req)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "past")
}

func TestValidator_Validate_RoundTripNoReturnDate(t *testing.T) {
	v := setupValidator()

	req := &models.FlightSearchRequest{
		FromAirport:      "JFK",
		ToAirport:        "LAX",
		DepartureDate:    "2025-12-15",
		TripType:         models.TripTypeRoundTrip,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	result := v.Validate(req)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "Return date is required")
}

func TestValidator_Validate_ReturnBeforeDeparture(t *testing.T) {
	v := setupValidator()

	req := &models.FlightSearchRequest{
		FromAirport:      "JFK",
		ToAirport:        "LAX",
		DepartureDate:    "2025-12-20",
		ReturnDate:       "2025-12-15",
		TripType:         models.TripTypeRoundTrip,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	result := v.Validate(req)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "before departure")
}

func TestValidator_Validate_TooManyPassengers(t *testing.T) {
	v := setupValidator()

	req := &models.FlightSearchRequest{
		FromAirport:        "JFK",
		ToAirport:          "LAX",
		DepartureDate:      "2025-12-15",
		TripType:           models.TripTypeOneWay,
		SeatClass:          models.SeatClassEconomy,
		PassengersAdults:   5,
		PassengersChildren: 5,
	}

	result := v.Validate(req)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "Maximum 9 passengers")
}

func TestValidator_Validate_InfantsExceedAdults(t *testing.T) {
	v := setupValidator()

	req := &models.FlightSearchRequest{
		FromAirport:        "JFK",
		ToAirport:          "LAX",
		DepartureDate:      "2025-12-15",
		TripType:           models.TripTypeOneWay,
		SeatClass:          models.SeatClassEconomy,
		PassengersAdults:   1,
		PassengersInfantsLap: 2,
	}

	result := v.Validate(req)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "infants cannot exceed")
}

func TestValidator_Validate_UnknownAirportWarning(t *testing.T) {
	v := setupValidator()

	futureDate := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	req := &models.FlightSearchRequest{
		FromAirport:      "JFK",
		ToAirport:        "XYZ",
		DepartureDate:    futureDate,
		TripType:         models.TripTypeOneWay,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	result := v.Validate(req)
	assert.True(t, result.Valid)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "not in our database")
}
