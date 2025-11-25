package models

import "time"

// FlightSearchResponse represents the response from a flight search.
type FlightSearchResponse struct {
	Request         *FlightSearchRequest `json:"request"`
	Flights         []*FlightResult      `json:"flights"`
	TotalResults    int                  `json:"total_results"`
	CheapestPrice   string               `json:"cheapest_price,omitempty"`
	SearchTimestamp time.Time            `json:"search_timestamp"`
}

// NewFlightSearchResponse creates a new FlightSearchResponse.
func NewFlightSearchResponse(req *FlightSearchRequest, flights []*FlightResult) *FlightSearchResponse {
	return &FlightSearchResponse{
		Request:         req,
		Flights:         flights,
		TotalResults:    len(flights),
		SearchTimestamp: time.Now(),
	}
}

// SetCheapestPrice calculates and sets the cheapest price from the flights.
func (r *FlightSearchResponse) SetCheapestPrice(price string) {
	r.CheapestPrice = price
}
