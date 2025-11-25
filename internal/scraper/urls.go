package scraper

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dvitali/flights-mcp/pkg/models"
)

// BuildGoogleFlightsURL builds a Google Flights search URL from a request.
func BuildGoogleFlightsURL(req *models.FlightSearchRequest) string {
	// Parse the departure date
	depDate, _ := time.Parse("2006-01-02", req.DepartureDate)
	dateStr := depDate.Format("Jan 2")

	// Build the search query
	var queryParts []string

	// Basic route
	queryParts = append(queryParts, fmt.Sprintf("flights from %s to %s", req.FromAirport, req.ToAirport))

	// Date
	queryParts = append(queryParts, fmt.Sprintf("on %s", dateStr))

	// Round trip
	if req.TripType == models.TripTypeRoundTrip && req.ReturnDate != "" {
		retDate, _ := time.Parse("2006-01-02", req.ReturnDate)
		queryParts = append(queryParts, fmt.Sprintf("returning %s", retDate.Format("Jan 2")))
	}

	query := strings.Join(queryParts, " ")

	// Build URL with query parameter
	baseURL := "https://www.google.com/travel/flights"
	params := url.Values{}
	params.Set("q", query)

	return baseURL + "?" + params.Encode()
}

// BuildDirectFlightsURL builds a direct Google Flights URL with structured parameters.
func BuildDirectFlightsURL(req *models.FlightSearchRequest) string {
	// This builds the more structured URL format:
	// https://www.google.com/travel/flights/search?tfs=...

	baseURL := "https://www.google.com/travel/flights/search"

	params := url.Values{}

	// Currency and language
	params.Set("curr", "USD")
	params.Set("hl", "en")

	// Seat class mapping
	seatMap := map[models.SeatClass]string{
		models.SeatClassEconomy:        "1",
		models.SeatClassPremiumEconomy: "2",
		models.SeatClassBusiness:       "3",
		models.SeatClassFirst:          "4",
	}
	if seat, ok := seatMap[req.SeatClass]; ok {
		params.Set("tfc", seat)
	}

	// Passengers
	if req.PassengersAdults > 0 {
		params.Set("tfa", fmt.Sprintf("%d", req.PassengersAdults))
	}

	return baseURL + "?" + params.Encode()
}

// BuildSimpleSearchURL builds a simple Google Flights URL that redirects to search.
func BuildSimpleSearchURL(from, to, date string) string {
	return fmt.Sprintf("https://www.google.com/travel/flights?q=flights+from+%s+to+%s+on+%s",
		url.QueryEscape(from),
		url.QueryEscape(to),
		url.QueryEscape(date))
}
