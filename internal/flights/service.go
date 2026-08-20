package flights

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/denysvitali/flights-mcp/internal/airports"
	"github.com/denysvitali/flights-mcp/internal/config"
	"github.com/denysvitali/flights-mcp/internal/scraper"
	"github.com/denysvitali/flights-mcp/pkg/models"
)

// Service provides flight search functionality.
type Service struct {
	scraper     scraper.Scraper
	airportDB   *airports.Database
	validator   *Validator
	rateLimiter *RateLimiter
	config      *config.Config
}

// NewService creates a new flight service.
func NewService(scr scraper.Scraper, db *airports.Database, cfg *config.Config) *Service {
	return &Service{
		scraper:     scr,
		airportDB:   db,
		validator:   NewValidator(db),
		rateLimiter: NewRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow),
		config:      cfg,
	}
}

// SearchFlights searches for flights based on the request.
func (s *Service) SearchFlights(ctx context.Context, req *models.FlightSearchRequest) (*models.FlightSearchResponse, error) {
	// Validate the request
	result := s.validator.Validate(req)
	if !result.Valid {
		return nil, NewValidationError(strings.Join(result.Errors, "; "))
	}

	// Check rate limit
	if !s.rateLimiter.Allow() {
		return nil, NewRateLimitError(s.config.RateLimitRequests, s.config.RateLimitWindow.String())
	}

	// Sanitize airport codes
	fromCode, _ := airports.SanitizeAirportCode(req.FromAirport)
	toCode, _ := airports.SanitizeAirportCode(req.ToAirport)

	// Create sanitized request
	sanitizedReq := &models.FlightSearchRequest{
		FromAirport:           fromCode,
		ToAirport:             toCode,
		DepartureDate:         req.DepartureDate,
		ReturnDate:            req.ReturnDate,
		TripType:              req.TripType,
		SeatClass:             req.SeatClass,
		PassengersAdults:      req.PassengersAdults,
		PassengersChildren:    req.PassengersChildren,
		PassengersInfantsSeat: req.PassengersInfantsSeat,
		PassengersInfantsLap:  req.PassengersInfantsLap,
	}

	log.Printf("Searching flights: %s -> %s on %s", fromCode, toCode, req.DepartureDate)

	// Perform the search with retries
	var lastErr error
	for attempt := 0; attempt < s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d/%d", attempt+1, s.config.MaxRetries)
			time.Sleep(s.config.RetryDelay)
		}

		scrapeResult, err := s.scraper.SearchFlights(ctx, sanitizedReq)
		if err == nil {
			// Success - build response
			response := s.buildResponse(sanitizedReq, scrapeResult)
			return response, nil
		}

		lastErr = err
		log.Printf("Search attempt %d failed: %v", attempt+1, err)

		// Check if error is retryable
		if ctx.Err() != nil {
			// Context cancelled or timed out
			break
		}
	}

	return nil, NewSearchError(lastErr)
}

// GetAirportInfo returns information about an airport.
func (s *Service) GetAirportInfo(code string) (*models.AirportInfo, error) {
	sanitized, err := airports.SanitizeAirportCode(code)
	if err != nil {
		return nil, NewAirportCodeError(code)
	}

	airport, ok := s.airportDB.Get(sanitized)
	if !ok {
		return nil, NewAirportNotFoundError(sanitized)
	}

	return airport, nil
}

// ValidateParams validates flight search parameters.
func (s *Service) ValidateParams(req *models.FlightSearchRequest) *ValidationResult {
	return s.validator.Validate(req)
}

// buildResponse builds a FlightSearchResponse from scrape results.
func (s *Service) buildResponse(req *models.FlightSearchRequest, result *scraper.ScrapeResult) *models.FlightSearchResponse {
	flights := result.Flights

	// Sort flights by price
	sort.Slice(flights, func(i, j int) bool {
		priceI := parsePrice(flights[i].Price)
		priceJ := parsePrice(flights[j].Price)
		return priceI < priceJ
	})

	response := models.NewFlightSearchResponse(req, flights)

	// Set cheapest price
	if result.CheapestPrice != "" {
		response.SetCheapestPrice(result.CheapestPrice)
	} else if len(flights) > 0 {
		response.SetCheapestPrice(flights[0].Price)
	}

	return response
}

// parsePrice parses a price string to a float for sorting.
func parsePrice(priceStr string) float64 {
	cleaned := strings.ReplaceAll(priceStr, "$", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSpace(cleaned)

	price, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 999999 // Sort unparseable prices to the end
	}
	return price
}

// FormatFlightResults formats flight search results for display.
func FormatFlightResults(response *models.FlightSearchResponse) string {
	if response == nil || len(response.Flights) == 0 {
		return fmt.Sprintf("No flights found for %s -> %s on %s",
			response.Request.FromAirport,
			response.Request.ToAirport,
			response.Request.DepartureDate)
	}

	var sb strings.Builder

	// Header
	sb.WriteString("**Flight Search Results**\n\n")
	sb.WriteString(fmt.Sprintf("**Route:** %s -> %s\n", response.Request.FromAirport, response.Request.ToAirport))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", response.Request.DepartureDate))
	sb.WriteString(fmt.Sprintf("**Passengers:** %d adults\n", response.Request.PassengersAdults))
	sb.WriteString(fmt.Sprintf("**Class:** %s\n", response.Request.SeatClass))
	sb.WriteString("\n")

	// Summary
	sb.WriteString(fmt.Sprintf("**%d flights found**\n", response.TotalResults))
	if response.CheapestPrice != "" {
		sb.WriteString(fmt.Sprintf("**Cheapest:** %s\n", response.CheapestPrice))
	}
	sb.WriteString("\n")

	// Flight details (limit to 10)
	limit := 10
	if len(response.Flights) < limit {
		limit = len(response.Flights)
	}

	for i := 0; i < limit; i++ {
		flight := response.Flights[i]
		status := "Flight"
		if flight.IsBest {
			status = "Best Flight"
		}

		stopsText := "Non-stop"
		if flight.Stops > 0 {
			stopsText = fmt.Sprintf("%d stop", flight.Stops)
			if flight.Stops > 1 {
				stopsText += "s"
			}
		} else if flight.Stops < 0 {
			stopsText = "Unknown"
		}

		sb.WriteString(fmt.Sprintf("**%s %d**\n", status, i+1))
		sb.WriteString(fmt.Sprintf("  Flight: %s\n", flight.FlightName))
		sb.WriteString(fmt.Sprintf("  Airline: %s\n", flight.Airline))
		sb.WriteString(fmt.Sprintf("  Time: %s -> %s\n", flight.DepartureTime, flight.ArrivalTime))
		sb.WriteString(fmt.Sprintf("  Duration: %s\n", flight.Duration))
		sb.WriteString(fmt.Sprintf("  Stops: %s\n", stopsText))
		sb.WriteString(fmt.Sprintf("  Price: %s\n", flight.Price))
		sb.WriteString("\n")
	}

	if len(response.Flights) > 10 {
		sb.WriteString(fmt.Sprintf("... and %d more flights\n\n", len(response.Flights)-10))
	}

	sb.WriteString(fmt.Sprintf("**Search performed at:** %s", response.SearchTimestamp.Format("2006-01-02 15:04:05 UTC")))

	return sb.String()
}

// FormatAirportInfo formats airport information for display.
func FormatAirportInfo(info *models.AirportInfo) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**%s - %s**\n\n", info.Code, info.Name))
	sb.WriteString(fmt.Sprintf("**Location:** %s, %s\n", info.City, info.Country))
	sb.WriteString(fmt.Sprintf("**Code:** %s\n", info.Code))

	return sb.String()
}
