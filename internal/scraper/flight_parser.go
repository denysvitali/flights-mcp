package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/denysvitali/flights-mcp/pkg/models"
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
	dataStr := string(data)

	// Skip the )]}' prefix if present
	dataStr = strings.TrimPrefix(dataStr, ")]}'")

	// Skip the length line
	if idx := strings.Index(dataStr, "\n"); idx != -1 {
		dataStr = strings.TrimSpace(dataStr[idx+1:])
	}

	// Parse outer JSON structure
	var outerData []interface{}
	if err := json.Unmarshal([]byte(dataStr), &outerData); err != nil {
		// Try line by line
		for _, line := range strings.Split(dataStr, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "[") {
				if err := json.Unmarshal([]byte(line), &outerData); err == nil {
					break
				}
			}
		}
		if outerData == nil {
			return nil, fmt.Errorf("failed to parse outer JSON: %w", err)
		}
	}

	// Google's response structure: [["wrb.fr", null, "[[flight_data]]", ...], ...]
	// The flight data is embedded as a string at position [0][2]
	var allFlights []*models.FlightResult

	for _, item := range outerData {
		arr, ok := item.([]interface{})
		if !ok || len(arr) < 3 {
			continue
		}

		// Check if this is a data wrapper (first element is "wrb.fr")
		if first, ok := arr[0].(string); ok && first == "wrb.fr" {
			// Element at index 2 is the embedded JSON string
			if embeddedStr, ok := arr[2].(string); ok && len(embeddedStr) > 100 {
				// Parse the embedded JSON
				var innerData interface{}
				if err := json.Unmarshal([]byte(embeddedStr), &innerData); err == nil {
					flights := extractFlights(innerData)
					allFlights = append(allFlights, flights...)
					log.Printf("Extracted %d flights from embedded data", len(flights))
				}
			}
		}
	}

	if len(allFlights) == 0 {
		// Fall back to searching the entire structure
		allFlights = extractFlights(outerData)
	}

	return allFlights, nil
}

// extractFlights searches for flight data in the parsed structure.
// Google's structure: embedded[2][0][X] or embedded[3][0][X] where X is a wrapper containing:
//   - [0] = flight data array
//   - [1][0] = [nil, price]
func extractFlights(data interface{}) []*models.FlightResult {
	var results []*models.FlightResult
	seenKeys := make(map[string]bool)

	arr, ok := data.([]interface{})
	if !ok {
		return nil
	}

	// Check indices 2 and 3 for flight groups
	for _, groupIdx := range []int{2, 3} {
		if groupIdx >= len(arr) {
			continue
		}

		group, ok := arr[groupIdx].([]interface{})
		if !ok || len(group) == 0 {
			continue
		}

		// group[0] contains the flights array
		flightsArr, ok := group[0].([]interface{})
		if !ok {
			continue
		}

		for _, flightWrapper := range flightsArr {
			wrapper, ok := flightWrapper.([]interface{})
			if !ok || len(wrapper) < 2 {
				continue
			}

			// wrapper[0] is the flight data, wrapper[1] contains price
			flightData, ok := wrapper[0].([]interface{})
			if !ok {
				continue
			}

			// Extract price from wrapper[1][0] = [nil, price]
			price := 0
			if priceArr, ok := wrapper[1].([]interface{}); ok && len(priceArr) > 0 {
				if pricePair, ok := priceArr[0].([]interface{}); ok && len(pricePair) == 2 {
					if pricePair[0] == nil {
						if p, ok := toInt(pricePair[1]); ok && p > 0 {
							price = p
						}
					}
				}
			}

			if flight := tryParseAsFlightWithPrice(flightData, price); flight != nil {
				key := fmt.Sprintf("%s-%s-%s-%s",
					flight.Airline, flight.DepartureTime, flight.ArrivalTime, flight.Price)
				if !seenKeys[key] {
					seenKeys[key] = true
					results = append(results, flight)
				}
			}
		}
	}

	// Fallback: recursive search if structured approach fails
	if len(results) == 0 {
		var search func(interface{}, int)
		search = func(v interface{}, depth int) {
			if depth > 15 {
				return
			}
			arr, ok := v.([]interface{})
			if !ok {
				return
			}
			if flight := tryParseAsFlightWithPrice(arr, 0); flight != nil {
				key := fmt.Sprintf("%s-%s-%s-%s",
					flight.Airline, flight.DepartureTime, flight.ArrivalTime, flight.Price)
				if !seenKeys[key] {
					seenKeys[key] = true
					results = append(results, flight)
				}
			}
			for _, item := range arr {
				search(item, depth+1)
			}
		}
		search(data, 0)
	}

	return results
}

// tryParseAsFlightWithPrice attempts to parse an array as a flight entry with an external price.
// Google's flight format:
// [0] "F9"           - Airline code
// [1] ["Frontier"]   - Airline name
// [2] [[leg1], [leg2], ...]  - Flight legs
// [3] "JFK"          - Origin airport
// [4] [2025, 12, 25] - Departure date
// [5] [6, 20]        - Departure time [hour, minute]
// [6] "LAX"          - Destination airport
// [7] [2025, 12, 25] - Arrival date
// [8] [14, 3]        - Arrival time [hour, minute]
// [9] 643            - Duration in minutes
func tryParseAsFlightWithPrice(arr []interface{}, price int) *models.FlightResult {
	if len(arr) < 10 {
		return nil
	}

	// Check for airline code pattern (2 chars at position 0)
	airlineCode, ok := arr[0].(string)
	if !ok || len(airlineCode) < 2 || len(airlineCode) > 3 {
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
	// This list includes major US, European, and international carriers
	knownAirlines := map[string]bool{
		// US airlines
		"Frontier": true, "JetBlue": true, "American": true, "Delta": true,
		"United": true, "Southwest": true, "Spirit": true, "Alaska": true,
		"Volaris": true, "Sun Country Airlines": true, "Hawaiian Airlines": true,
		// European airlines
		"Air France": true, "SWISS": true, "Lufthansa": true, "British Airways": true,
		"KLM": true, "Iberia": true, "Condor": true, "Ryanair": true, "easyJet": true,
		"Vueling": true, "TAP Portugal": true, "SAS": true, "Norwegian": true,
		"Finnair": true, "Austrian": true, "Brussels Airlines": true, "LOT Polish Airlines": true,
		"Aer Lingus": true, "Eurowings": true, "Transavia": true, "Wizz Air": true,
		// Middle East & Asia
		"Emirates": true, "Qatar Airways": true, "Etihad": true, "Turkish Airlines": true,
		"Singapore Airlines": true, "Cathay Pacific": true, "ANA": true, "JAL": true,
		"Korean Air": true, "Asiana Airlines": true, "Air China": true, "China Eastern": true,
		"China Southern": true, "Hainan Airlines": true, "Thai Airways": true,
		// Other international
		"Air Canada": true, "WestJet": true, "Qantas": true, "Virgin Atlantic": true,
		"Virgin Australia": true, "LATAM": true, "Avianca": true, "Copa Airlines": true,
		"Aeromexico": true, "Air New Zealand": true, "South African Airways": true,
	}
	if !knownAirlines[airlineName] {
		return nil
	}

	var origin, destination string
	var depTime, arrTime [2]int
	var duration int

	// Origin at [3]
	if o, ok := arr[3].(string); ok && len(o) == 3 {
		origin = o
	}

	// Destination at [6]
	if d, ok := arr[6].(string); ok && len(d) == 3 {
		destination = d
	}

	// Departure time at [5]
	if len(arr) > 5 && arr[5] != nil {
		depTime = parseTimeArray(arr[5])
	}

	// Arrival time at [8]
	if len(arr) > 8 && arr[8] != nil {
		arrTime = parseTimeArray(arr[8])
	}

	// Duration at [9]
	if len(arr) > 9 {
		if d, ok := toInt(arr[9]); ok && d > 0 {
			duration = d
		}
	}

	// Validate we have minimum required data
	if origin == "" || destination == "" {
		return nil
	}

	// Number of stops = number of legs - 1
	stops := 0
	if legs, ok := arr[2].([]interface{}); ok {
		stops = len(legs) - 1
	}

	// Format times
	depTimeStr := formatTimeHM(depTime[0], depTime[1])
	arrTimeStr := formatTimeHM(arrTime[0], arrTime[1])

	// Format duration
	durationStr := "N/A"
	if duration > 0 {
		durationStr = fmt.Sprintf("%d hr %d min", duration/60, duration%60)
	}

	// Format price
	priceStr := "Price unavailable"
	if price > 0 {
		priceStr = fmt.Sprintf("CHF %d", price)
	}

	return &models.FlightResult{
		FlightName:    airlineCode,
		Airline:       airlineName,
		DepartureTime: depTimeStr,
		ArrivalTime:   arrTimeStr,
		Duration:      durationStr,
		Stops:         stops,
		Price:         priceStr,
		IsBest:        false,
	}
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

// parseTimeArray extracts hour and minute from a time array.
// Handles various formats:
// - [hour, minute] - normal format
// - [hour] - hour only, minute defaults to 0
// - [nil, minute] - hour is 0 (midnight/12 AM)
func parseTimeArray(v interface{}) [2]int {
	t, ok := v.([]interface{})
	if !ok || len(t) == 0 {
		return [2]int{0, 0}
	}

	var hour, minute int

	// Parse hour (can be nil for midnight)
	if t[0] != nil {
		h, ok := toInt(t[0])
		if ok {
			hour = h
		}
	}
	// If hour is nil, it stays 0 (midnight)

	// Parse minute if present
	if len(t) >= 2 && t[1] != nil {
		m, ok := toInt(t[1])
		if ok {
			minute = m
		}
	}

	return [2]int{hour, minute}
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
