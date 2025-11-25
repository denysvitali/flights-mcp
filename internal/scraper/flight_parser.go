package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/dvitali/flights-mcp/pkg/models"
)

// GoogleFlightData represents a single flight offering from Google Flights.
// The data comes as nested arrays, so we parse positionally.
type GoogleFlightData struct {
	AirlineCode   string
	AirlineName   string
	Origin        string
	Destination   string
	DepartureTime [2]int // [hour, minute]
	ArrivalTime   [2]int // [hour, minute]
	DurationMins  int
	Legs          []FlightLeg
	Price         int // in local currency (cents or whole units)
}

// FlightLeg represents one segment of a flight.
type FlightLeg struct {
	Origin        string
	Destination   string
	DepartureTime [2]int
	ArrivalTime   [2]int
	DurationMins  int
	Aircraft      string
	FlightNumber  string
}

// ParseGoogleFlightsData parses the Google Flights API response.
func ParseGoogleFlightsData(data []byte) ([]*models.FlightResult, error) {
	// Unescape the data (Google's format has escaped quotes within JSON strings)
	dataStr := string(data)
	dataStr = strings.ReplaceAll(dataStr, `\"`, `"`)
	dataStr = strings.ReplaceAll(dataStr, `\\`, `\`)

	// Skip the )]}' prefix if present
	if strings.HasPrefix(dataStr, ")]}'") {
		dataStr = dataStr[4:]
	}

	// Skip the length line
	if idx := strings.Index(dataStr, "\n"); idx != -1 {
		dataStr = strings.TrimSpace(dataStr[idx+1:])
	}

	// Parse as generic JSON
	var rawData []interface{}
	if err := json.Unmarshal([]byte(dataStr), &rawData); err != nil {
		// Try parsing line by line
		lines := strings.Split(dataStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "[") {
				if err := json.Unmarshal([]byte(line), &rawData); err == nil {
					break
				}
			}
		}
		if rawData == nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	// Extract flights from the nested structure
	flights := extractFlights(rawData)
	return flights, nil
}

// extractFlights recursively searches for flight data.
func extractFlights(data interface{}) []*models.FlightResult {
	var results []*models.FlightResult
	seenKeys := make(map[string]bool)

	var search func(interface{}, int)
	search = func(v interface{}, depth int) {
		if depth > 20 {
			return // Prevent infinite recursion
		}

		arr, ok := v.([]interface{})
		if !ok {
			return
		}

		// Try to parse this array as a flight
		if flight := tryParseAsFlight(arr); flight != nil {
			// Create unique key to avoid duplicates
			key := fmt.Sprintf("%s-%s-%d:%02d-%d",
				flight.Airline, flight.DepartureTime, flight.ArrivalTime, flight.Price)
			if !seenKeys[key] {
				seenKeys[key] = true
				results = append(results, flight)
			}
		}

		// Recurse into array elements
		for _, item := range arr {
			search(item, depth+1)
		}
	}

	search(data, 0)
	return results
}

// tryParseAsFlight attempts to parse an array as a flight entry.
// Google's flight format is:
// ["F9", ["Frontier"], [...legs...], "JFK", [date], [time], "LAX", [date], [time], duration, ..., [[null, price]]]
func tryParseAsFlight(arr []interface{}) *models.FlightResult {
	if len(arr) < 10 {
		return nil
	}

	// Check for airline code pattern (2 chars at position 0)
	airlineCode, ok := arr[0].(string)
	if !ok || len(airlineCode) != 2 {
		return nil
	}

	// Check for airline name array at position 1
	airlineArr, ok := arr[1].([]interface{})
	if !ok || len(airlineArr) == 0 {
		return nil
	}
	airlineName, ok := airlineArr[0].(string)
	if !ok {
		return nil
	}

	// Look for known airlines to validate
	knownAirlines := map[string]bool{
		"Frontier": true, "JetBlue": true, "American": true, "Delta": true,
		"United": true, "Southwest": true, "Spirit": true, "Alaska": true,
		"Volaris": true, "Sun Country Airlines": true,
	}
	if !knownAirlines[airlineName] {
		return nil
	}

	// Find origin airport (3-letter string)
	var origin, destination string
	var depTime, arrTime [2]int
	var duration int
	var price int

	// Scan the array for expected patterns
	for i, item := range arr {
		switch v := item.(type) {
		case string:
			// 3-letter codes are likely airports
			if len(v) == 3 && v == strings.ToUpper(v) {
				if origin == "" {
					origin = v
				} else if destination == "" && v != origin {
					destination = v
				}
			}
		case []interface{}:
			// [hour, minute] for times
			if len(v) == 2 {
				h, hOk := toInt(v[0])
				m, mOk := toInt(v[1])
				if hOk && mOk && h >= 0 && h <= 23 && m >= 0 && m <= 59 {
					if depTime[0] == 0 && depTime[1] == 0 {
						depTime = [2]int{h, m}
					} else if arrTime[0] == 0 && arrTime[1] == 0 {
						arrTime = [2]int{h, m}
					}
				}
			}
			// [[null, price]] for price
			if len(v) == 1 {
				if inner, ok := v[0].([]interface{}); ok && len(inner) == 2 {
					if inner[0] == nil {
						if p, ok := toInt(inner[1]); ok && p > 50 && p < 50000 {
							price = p
						}
					}
				}
			}
		case float64:
			// Duration in minutes (typically 100-2000)
			if i > 5 && int(v) >= 60 && int(v) <= 2000 && duration == 0 {
				duration = int(v)
			}
		}
	}

	// Validate we have minimum required data
	if origin == "" || destination == "" || price == 0 {
		return nil
	}

	// Count stops by looking for intermediate airports
	stops := countStops(arr, origin, destination)

	// Format times
	depTimeStr := formatTimeHM(depTime[0], depTime[1])
	arrTimeStr := formatTimeHM(arrTime[0], arrTime[1])

	// Format duration
	durationStr := "N/A"
	if duration > 0 {
		durationStr = fmt.Sprintf("%d hr %d min", duration/60, duration%60)
	}

	return &models.FlightResult{
		FlightName:    airlineCode,
		Airline:       airlineName,
		DepartureTime: depTimeStr,
		ArrivalTime:   arrTimeStr,
		Duration:      durationStr,
		Stops:         stops,
		Price:         fmt.Sprintf("CHF %d", price),
		IsBest:        false,
	}
}

// countStops counts intermediate airports in the flight data.
func countStops(arr []interface{}, origin, destination string) int {
	airports := make(map[string]bool)

	var findAirports func(interface{})
	findAirports = func(v interface{}) {
		switch item := v.(type) {
		case string:
			if len(item) == 3 && item == strings.ToUpper(item) {
				airports[item] = true
			}
		case []interface{}:
			for _, sub := range item {
				findAirports(sub)
			}
		}
	}
	findAirports(arr)

	// Remove origin and destination
	delete(airports, origin)
	delete(airports, destination)

	// Common non-airport 3-letter codes to ignore
	delete(airports, "USA")
	delete(airports, "USD")
	delete(airports, "CHF")
	delete(airports, "EUR")

	return len(airports)
}

// toInt converts an interface{} to int.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}

// formatTimeHM formats hour and minute as "H:MM AM/PM".
func formatTimeHM(hour, minute int) string {
	if hour == 0 && minute == 0 {
		return "N/A"
	}
	ampm := "AM"
	displayHour := hour
	if hour >= 12 {
		ampm = "PM"
		if hour > 12 {
			displayHour = hour - 12
		}
	}
	if displayHour == 0 {
		displayHour = 12
	}
	return fmt.Sprintf("%d:%02d %s", displayHour, minute, ampm)
}

// ParseFlightsFromResponse is the main entry point for parsing Google Flights data.
func ParseFlightsFromResponse(data []byte) []*models.FlightResult {
	flights, err := ParseGoogleFlightsData(data)
	if err != nil {
		log.Printf("Error parsing flight data: %v", err)
		return nil
	}
	return flights
}
