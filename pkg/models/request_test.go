package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFlightSearchRequest(t *testing.T) {
	req := NewFlightSearchRequest("JFK", "LAX", "2025-12-15")

	assert.Equal(t, "JFK", req.FromAirport)
	assert.Equal(t, "LAX", req.ToAirport)
	assert.Equal(t, "2025-12-15", req.DepartureDate)
	assert.Equal(t, TripTypeOneWay, req.TripType)
	assert.Equal(t, SeatClassEconomy, req.SeatClass)
	assert.Equal(t, 1, req.PassengersAdults)
	assert.Equal(t, 0, req.PassengersChildren)
}

func TestIsValidTripType(t *testing.T) {
	assert.True(t, IsValidTripType(TripTypeOneWay))
	assert.True(t, IsValidTripType(TripTypeRoundTrip))
	assert.False(t, IsValidTripType("invalid"))
	assert.False(t, IsValidTripType(""))
}

func TestIsValidSeatClass(t *testing.T) {
	assert.True(t, IsValidSeatClass(SeatClassEconomy))
	assert.True(t, IsValidSeatClass(SeatClassPremiumEconomy))
	assert.True(t, IsValidSeatClass(SeatClassBusiness))
	assert.True(t, IsValidSeatClass(SeatClassFirst))
	assert.False(t, IsValidSeatClass("invalid"))
	assert.False(t, IsValidSeatClass(""))
}
