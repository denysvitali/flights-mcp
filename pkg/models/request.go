package models

// TripType represents the type of trip.
type TripType string

const (
	TripTypeOneWay    TripType = "one-way"
	TripTypeRoundTrip TripType = "round-trip"
)

// SeatClass represents the class of seat.
type SeatClass string

const (
	SeatClassEconomy        SeatClass = "economy"
	SeatClassPremiumEconomy SeatClass = "premium-economy"
	SeatClassBusiness       SeatClass = "business"
	SeatClassFirst          SeatClass = "first"
)

// FlightSearchRequest represents a request to search for flights.
type FlightSearchRequest struct {
	FromAirport           string    `json:"from_airport"`
	ToAirport             string    `json:"to_airport"`
	DepartureDate         string    `json:"departure_date"`
	ReturnDate            string    `json:"return_date,omitempty"`
	TripType              TripType  `json:"trip_type"`
	SeatClass             SeatClass `json:"seat_class"`
	PassengersAdults      int       `json:"passengers_adults"`
	PassengersChildren    int       `json:"passengers_children"`
	PassengersInfantsSeat int       `json:"passengers_infants_in_seat"`
	PassengersInfantsLap  int       `json:"passengers_infants_on_lap"`
}

// NewFlightSearchRequest creates a new FlightSearchRequest with defaults.
func NewFlightSearchRequest(from, to, departureDate string) *FlightSearchRequest {
	return &FlightSearchRequest{
		FromAirport:           from,
		ToAirport:             to,
		DepartureDate:         departureDate,
		TripType:              TripTypeOneWay,
		SeatClass:             SeatClassEconomy,
		PassengersAdults:      1,
		PassengersChildren:    0,
		PassengersInfantsSeat: 0,
		PassengersInfantsLap:  0,
	}
}

// IsValidTripType checks if the trip type is valid.
func IsValidTripType(t TripType) bool {
	return t == TripTypeOneWay || t == TripTypeRoundTrip
}

// IsValidSeatClass checks if the seat class is valid.
func IsValidSeatClass(s SeatClass) bool {
	switch s {
	case SeatClassEconomy, SeatClassPremiumEconomy, SeatClassBusiness, SeatClassFirst:
		return true
	default:
		return false
	}
}
