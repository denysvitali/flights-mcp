package scraper

import (
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/dvitali/flights-mcp/pkg/models"
)

// Parser extracts flight data from HTML.
type Parser struct {
	// Regex to extract price from aria-label: "From 375 Swiss francs" or "From $375"
	priceRegex *regexp.Regexp
	// Regex to extract price with currency symbol
	currencyPriceRegex *regexp.Regexp
}

// NewParser creates a new Parser.
func NewParser() *Parser {
	return &Parser{
		priceRegex:         regexp.MustCompile(`From\s+(\d+(?:,\d+)?)\s+(\w+\s*\w*)`),
		currencyPriceRegex: regexp.MustCompile(`[\$€£]\s*(\d+(?:,\d+)*)`),
	}
}

// Parse extracts flight results from HTML content.
func (p *Parser) Parse(html string) (*ScrapeResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var flights []*models.FlightResult

	// Strategy 1: Parse from aria-label attributes (most reliable for Google Flights)
	flights = p.parseFromAriaLabels(doc)
	log.Printf("Strategy 1 (aria-labels): found %d flights", len(flights))

	// Strategy 2: Try traditional selectors if aria-labels didn't work
	if len(flights) == 0 {
		flights = p.parseFromSelectors(doc)
		log.Printf("Strategy 2 (selectors): found %d flights", len(flights))
	}

	// Calculate cheapest price
	cheapestPrice := p.findCheapestPrice(flights)

	return &ScrapeResult{
		Flights:       flights,
		CheapestPrice: cheapestPrice,
	}, nil
}

// parseFromAriaLabels extracts flight data from aria-label attributes.
// Google Flights puts comprehensive flight info in aria-labels like:
// "From 375 Swiss francs round trip total. 1 stop flight with Frontier..."
func (p *Parser) parseFromAriaLabels(doc *goquery.Document) []*models.FlightResult {
	var flights []*models.FlightResult
	seen := make(map[string]bool)

	// Find elements with flight information in aria-label
	doc.Find("[aria-label*='flight with']").Each(func(i int, s *goquery.Selection) {
		label, exists := s.Attr("aria-label")
		if !exists || label == "" {
			return
		}

		// Skip if we've seen this exact label
		if seen[label] {
			return
		}
		seen[label] = true

		flight := p.parseAriaLabel(label)
		if flight != nil {
			flights = append(flights, flight)
		}
	})

	// Also try elements with "Select flight" that have price info
	doc.Find("[aria-label*='round trip total']").Each(func(i int, s *goquery.Selection) {
		label, exists := s.Attr("aria-label")
		if !exists || label == "" {
			return
		}

		if seen[label] {
			return
		}
		seen[label] = true

		flight := p.parseAriaLabel(label)
		if flight != nil {
			flights = append(flights, flight)
		}
	})

	return flights
}

// parseAriaLabel parses a single aria-label string into a FlightResult.
func (p *Parser) parseAriaLabel(label string) *models.FlightResult {
	// Example: "From 375 Swiss francs round trip total. 1 stop flight with Frontier.
	// Leaves John F. Kennedy International Airport at 6:20 AM on Thursday, December 25
	// and arrives at Los Angeles International Airport at 2:03 PM on Thursday, December 25.
	// Total duration 10 hr 43 min. Layover..."

	flight := &models.FlightResult{
		FlightName: "Flight",
		IsBest:     false,
	}

	// Extract price
	priceMatch := p.priceRegex.FindStringSubmatch(label)
	if len(priceMatch) >= 3 {
		price := priceMatch[1]
		currency := priceMatch[2]
		// Normalize currency
		if strings.Contains(strings.ToLower(currency), "franc") {
			flight.Price = "CHF " + price
		} else if strings.Contains(strings.ToLower(currency), "dollar") || strings.Contains(strings.ToLower(currency), "usd") {
			flight.Price = "$" + price
		} else if strings.Contains(strings.ToLower(currency), "euro") {
			flight.Price = "€" + price
		} else {
			flight.Price = currency + " " + price
		}
	}

	// Skip if no price found
	if flight.Price == "" {
		// Check for "price is unavailable"
		if strings.Contains(label, "price is unavailable") {
			flight.Price = "Price unavailable"
		} else {
			return nil
		}
	}

	// Extract airline (after "flight with")
	airlineRegex := regexp.MustCompile(`flight with\s+([^.]+)\.`)
	airlineMatch := airlineRegex.FindStringSubmatch(label)
	if len(airlineMatch) >= 2 {
		flight.Airline = strings.TrimSpace(airlineMatch[1])
	} else {
		flight.Airline = "Various Airlines"
	}

	// Extract stops
	if strings.Contains(label, "Nonstop") || strings.Contains(label, "nonstop") {
		flight.Stops = 0
	} else {
		stopsRegex := regexp.MustCompile(`(\d+)\s+stop`)
		stopsMatch := stopsRegex.FindStringSubmatch(label)
		if len(stopsMatch) >= 2 {
			flight.Stops, _ = strconv.Atoi(stopsMatch[1])
		} else {
			flight.Stops = -1 // Unknown
		}
	}

	// Extract all times from the label - format: "X:XX AM" or "X:XX PM"
	// Note: Google may use non-breaking spaces or different formats
	timeRegex := regexp.MustCompile(`(\d{1,2}:\d{2}\s*[AP]M)`)
	times := timeRegex.FindAllString(label, -1)

	// Debug: log first label to see format
	if len(times) == 0 && strings.Contains(label, ":") {
		log.Printf("DEBUG: No times found in label containing colon. First 200 chars: %s", label[:min(200, len(label))])
	}

	if len(times) >= 1 {
		flight.DepartureTime = times[0]
	} else {
		flight.DepartureTime = "N/A"
	}

	if len(times) >= 2 {
		flight.ArrivalTime = times[1]
	} else {
		flight.ArrivalTime = "N/A"
	}

	// Extract duration
	durationRegex := regexp.MustCompile(`Total duration\s+(\d+\s*hr\s*\d*\s*min|\d+\s*hr|\d+\s*min)`)
	durationMatch := durationRegex.FindStringSubmatch(label)
	if len(durationMatch) >= 2 {
		flight.Duration = durationMatch[1]
	} else {
		flight.Duration = "N/A"
	}

	// Check if this is marked as best
	if strings.Contains(strings.ToLower(label), "best") {
		flight.IsBest = true
	}

	return flight
}

// parseFromSelectors tries to extract flights using DOM selectors.
func (p *Parser) parseFromSelectors(doc *goquery.Document) []*models.FlightResult {
	var flights []*models.FlightResult

	// Try to find flight cards by common patterns
	doc.Find("li[data-ved], [role='listitem']").Each(func(i int, s *goquery.Selection) {
		text := s.Text()

		// Must have some price indicator
		if !strings.Contains(text, "$") && !strings.Contains(text, "€") &&
		   !strings.Contains(text, "£") && !strings.Contains(text, "franc") {
			return
		}

		// Try to extract basic info
		flight := &models.FlightResult{
			FlightName: "Flight",
			Airline:    "See details",
		}

		// Find price
		priceMatches := p.currencyPriceRegex.FindAllString(text, -1)
		if len(priceMatches) > 0 {
			// Use the first reasonable price (not $1, $2, etc.)
			for _, price := range priceMatches {
				cleaned := strings.ReplaceAll(price, ",", "")
				cleaned = strings.ReplaceAll(cleaned, "$", "")
				cleaned = strings.ReplaceAll(cleaned, "€", "")
				cleaned = strings.ReplaceAll(cleaned, "£", "")
				if val, err := strconv.Atoi(cleaned); err == nil && val > 50 {
					flight.Price = price
					break
				}
			}
		}

		if flight.Price != "" {
			flights = append(flights, flight)
		}
	})

	return flights
}

// findCheapestPrice finds the cheapest price from the flight results.
func (p *Parser) findCheapestPrice(flights []*models.FlightResult) string {
	if len(flights) == 0 {
		return ""
	}

	var cheapest float64 = -1
	var cheapestStr string

	for _, flight := range flights {
		price := p.parsePrice(flight.Price)
		if price > 0 && (cheapest < 0 || price < cheapest) {
			cheapest = price
			cheapestStr = flight.Price
		}
	}

	return cheapestStr
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parsePrice parses a price string to a float.
func (p *Parser) parsePrice(priceStr string) float64 {
	// Remove currency symbols and words
	cleaned := priceStr
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	cleaned = strings.ReplaceAll(cleaned, "€", "")
	cleaned = strings.ReplaceAll(cleaned, "£", "")
	cleaned = strings.ReplaceAll(cleaned, "CHF", "")
	cleaned = strings.ReplaceAll(cleaned, "Swiss francs", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSpace(cleaned)

	price, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return -1
	}
	return price
}
