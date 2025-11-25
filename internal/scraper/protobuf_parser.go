package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/dvitali/flights-mcp/pkg/models"
)

// ParseGoogleFlightsResponse parses the Google Flights protobuf-as-JSON response.
func ParseGoogleFlightsResponse(data []byte) ([]*models.FlightResult, error) {
	// Google's response starts with )]}' followed by a length, then the actual data
	dataStr := string(data)

	// Remove the )]}' prefix if present
	if strings.HasPrefix(dataStr, ")]}'") {
		dataStr = dataStr[4:]
	}

	// Skip the length line
	if idx := strings.Index(dataStr, "\n"); idx != -1 {
		dataStr = strings.TrimSpace(dataStr[idx+1:])
	}

	// Parse as JSON array
	var rawData interface{}
	if err := json.Unmarshal([]byte(dataStr), &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract flights from the nested structure
	flights := extractFlightsFromData(rawData)
	log.Printf("Extracted %d flights from protobuf data", len(flights))

	return flights, nil
}

// extractFlightsFromData recursively searches for flight data in the parsed JSON.
func extractFlightsFromData(data interface{}) []*models.FlightResult {
	var flights []*models.FlightResult

	switch v := data.(type) {
	case []interface{}:
		// Check if this looks like a flight entry
		if flight := tryParseFlightEntry(v); flight != nil {
			flights = append(flights, flight)
		}

		// Recurse into array elements
		for _, item := range v {
			flights = append(flights, extractFlightsFromData(item)...)
		}

	case map[string]interface{}:
		for _, val := range v {
			flights = append(flights, extractFlightsFromData(val)...)
		}
	}

	return flights
}

// tryParseFlightEntry checks if an array looks like a flight entry and parses it.
func tryParseFlightEntry(arr []interface{}) *models.FlightResult {
	// Look for the characteristic pattern:
	// - Contains airline code like "F9", "B6", "AA", etc.
	// - Contains airport codes like "JFK", "LAX"
	// - Contains time arrays like [6, 20]
	// - Contains date arrays like [2025, 12, 25]

	if len(arr) < 5 {
		return nil
	}

	// Try to find airline info - it's usually in a nested array like ["F9", ["Frontier"]]
	airlineCode := ""
	airlineName := ""

	// Try to find price - usually appears as [[null, 374]] or similar
	var price float64

	// Try to find times
	var departureTime, arrivalTime string

	// Scan the array for known patterns
	jsonStr, _ := json.Marshal(arr)
	strData := string(jsonStr)

	// Check if this contains flight-like data
	hasAirline := false
	hasPrice := false

	// Look for airline pattern: ["XX",["Airline Name"]]
	airlineRegex := regexp.MustCompile(`\["([A-Z0-9]{2})",\["([^"]+)"\]\]`)
	if matches := airlineRegex.FindStringSubmatch(strData); len(matches) >= 3 {
		airlineCode = matches[1]
		airlineName = matches[2]
		hasAirline = true
	}

	// Look for price pattern: [[null,XXX]] where XXX is the price
	priceRegex := regexp.MustCompile(`\[\[null,(\d+)\]`)
	if matches := priceRegex.FindStringSubmatch(strData); len(matches) >= 2 {
		fmt.Sscanf(matches[1], "%f", &price)
		hasPrice = true
	}

	// Look for time patterns: [HH,MM]
	timeRegex := regexp.MustCompile(`\[(\d{1,2}),(\d{1,2})\]`)
	timeMatches := timeRegex.FindAllStringSubmatch(strData, -1)

	// Filter for likely times (hours 0-23, minutes 0-59)
	var times []string
	for _, match := range timeMatches {
		if len(match) >= 3 {
			hour := 0
			minute := 0
			fmt.Sscanf(match[1], "%d", &hour)
			fmt.Sscanf(match[2], "%d", &minute)
			if hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 {
				// Format as time
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
				times = append(times, fmt.Sprintf("%d:%02d %s", displayHour, minute, ampm))
			}
		}
	}

	if len(times) >= 2 {
		departureTime = times[0]
		arrivalTime = times[1]
	}

	// Look for airport codes (3 uppercase letters) to count stops
	airportRegex := regexp.MustCompile(`"([A-Z]{3})"`)
	airportMatches := airportRegex.FindAllStringSubmatch(strData, -1)
	var airports []string
	seenAirports := make(map[string]bool)
	for _, match := range airportMatches {
		if len(match) >= 2 && !seenAirports[match[1]] {
			airports = append(airports, match[1])
			seenAirports[match[1]] = true
		}
	}

	// Only return a flight if we have the essential data
	if !hasAirline || !hasPrice || price < 50 {
		return nil
	}

	// Look for duration - often appears as a number in the 100-1000 range for minutes
	durationRegex := regexp.MustCompile(`[,\[](\d{3,4})[,\]]`)
	durationMatches := durationRegex.FindAllStringSubmatch(strData, -1)
	var duration string
	for _, match := range durationMatches {
		if len(match) >= 2 {
			mins := 0
			fmt.Sscanf(match[1], "%d", &mins)
			if mins >= 60 && mins <= 2000 {
				hours := mins / 60
				remainMins := mins % 60
				duration = fmt.Sprintf("%d hr %d min", hours, remainMins)
				break
			}
		}
	}

	// Determine number of stops
	stops := 0
	if strings.Contains(strData, "\"ATL\"") || strings.Contains(strData, "Layover") {
		stops = 1 // Has a connection
	}
	// Count intermediate airports
	if len(airports) > 2 {
		stops = len(airports) - 2
	}

	return &models.FlightResult{
		FlightName:    airlineCode,
		Airline:       airlineName,
		DepartureTime: departureTime,
		ArrivalTime:   arrivalTime,
		Duration:      duration,
		Stops:         stops,
		Price:         fmt.Sprintf("CHF %.0f", price),
		IsBest:        false,
	}
}

// ParseFlightsFromRawData is an alternative parser that looks for specific patterns.
func ParseFlightsFromRawData(data []byte) []*models.FlightResult {
	var flights []*models.FlightResult
	seen := make(map[string]bool)

	dataStr := string(data)

	// Pattern to find flight entries with airline and price
	// The data has escaped quotes: [\"F9\",[\"Frontier\"]]

	// Find all airline occurrences (handles both escaped and unescaped quotes)
	airlineRegex := regexp.MustCompile(`\[\\?"([A-Z0-9]{2})\\?",\[\\?"([^"\\]+)\\?"\]\]`)
	airlineMatches := airlineRegex.FindAllStringSubmatchIndex(dataStr, -1)

	for _, matchIdx := range airlineMatches {
		if len(matchIdx) < 6 {
			continue
		}

		// Extract the context around this airline (next ~2000 chars)
		start := matchIdx[0]
		end := start + 2000
		if end > len(dataStr) {
			end = len(dataStr)
		}
		context := dataStr[start:end]

		airlineCode := dataStr[matchIdx[2]:matchIdx[3]]
		airlineName := dataStr[matchIdx[4]:matchIdx[5]]

		// Find price in this context - handles [[null,374] or [null,374]
		priceRegex := regexp.MustCompile(`\[null,(\d+)\]`)
		priceMatch := priceRegex.FindStringSubmatch(context)
		if len(priceMatch) < 2 {
			continue
		}
		var price float64
		fmt.Sscanf(priceMatch[1], "%f", &price)
		if price < 50 {
			continue
		}

		// Find times in this context [H,M] - escaped or not
		timeRegex := regexp.MustCompile(`\[(\d{1,2}),(\d{1,2})\]`)
		timeMatches := timeRegex.FindAllStringSubmatch(context, 20)

		var depTime, arrTime string
		for i, tm := range timeMatches {
			if len(tm) >= 3 {
				h, m := 0, 0
				fmt.Sscanf(tm[1], "%d", &h)
				fmt.Sscanf(tm[2], "%d", &m)
				if h >= 0 && h <= 23 && m >= 0 && m <= 59 {
					timeStr := formatTime(h, m)
					if i == 0 {
						depTime = timeStr
					} else if arrTime == "" {
						arrTime = timeStr
					}
				}
			}
		}

		// Create unique key to avoid duplicates
		key := fmt.Sprintf("%s-%s-%.0f", airlineCode, depTime, price)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Count stops - look for intermediate airports
		stops := 0
		if strings.Count(context, `"ATL"`) > 0 || strings.Count(context, `"ORD"`) > 0 ||
		   strings.Count(context, `"DFW"`) > 0 || strings.Count(context, `"DEN"`) > 0 {
			stops = 1
		}

		// Find duration
		duration := ""
		durationRegex := regexp.MustCompile(`,(\d{3,4}),`)
		if dm := durationRegex.FindStringSubmatch(context); len(dm) >= 2 {
			mins := 0
			fmt.Sscanf(dm[1], "%d", &mins)
			if mins >= 100 && mins <= 2000 {
				duration = fmt.Sprintf("%d hr %d min", mins/60, mins%60)
			}
		}

		flights = append(flights, &models.FlightResult{
			FlightName:    airlineCode,
			Airline:       airlineName,
			DepartureTime: depTime,
			ArrivalTime:   arrTime,
			Duration:      duration,
			Stops:         stops,
			Price:         fmt.Sprintf("CHF %.0f", price),
			IsBest:        false,
		})
	}

	return flights
}

func formatTime(hour, minute int) string {
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
