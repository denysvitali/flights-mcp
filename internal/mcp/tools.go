package mcp

import (
	"context"
	"errors"
	"log"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/dvitali/flights-mcp/internal/flights"
	"github.com/dvitali/flights-mcp/pkg/models"
)

// handleSearchFlights handles the search_flights tool call.
func (s *Server) handleSearchFlights(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract required parameters
	fromAirport, err := request.RequireString("from_airport")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: from_airport"), nil
	}

	toAirport, err := request.RequireString("to_airport")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: to_airport"), nil
	}

	departureDate, err := request.RequireString("departure_date")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: departure_date"), nil
	}

	// Extract optional parameters with defaults
	returnDate := request.GetString("return_date", "")
	tripType := request.GetString("trip_type", "one-way")
	seatClass := request.GetString("seat_class", "economy")
	passengersAdults := request.GetInt("passengers_adults", 1)
	passengersChildren := request.GetInt("passengers_children", 0)
	passengersInfantsSeat := request.GetInt("passengers_infants_in_seat", 0)
	passengersInfantsLap := request.GetInt("passengers_infants_on_lap", 0)

	log.Printf("Flight search request: %s -> %s on %s", fromAirport, toAirport, departureDate)

	// Build the search request
	searchReq := &models.FlightSearchRequest{
		FromAirport:           fromAirport,
		ToAirport:             toAirport,
		DepartureDate:         departureDate,
		ReturnDate:            returnDate,
		TripType:              models.TripType(tripType),
		SeatClass:             models.SeatClass(seatClass),
		PassengersAdults:      passengersAdults,
		PassengersChildren:    passengersChildren,
		PassengersInfantsSeat: passengersInfantsSeat,
		PassengersInfantsLap:  passengersInfantsLap,
	}

	// Perform the search
	response, err := s.flightService.SearchFlights(ctx, searchReq)
	if err != nil {
		return formatErrorResult(err), nil
	}

	// Format and return results
	formattedResult := flights.FormatFlightResults(response)
	return mcp.NewToolResultText(formattedResult), nil
}

// handleGetAirportInfo handles the get_airport_info tool call.
func (s *Server) handleGetAirportInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	airportCode, err := request.RequireString("airport_code")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: airport_code"), nil
	}

	log.Printf("Airport info request: %s", airportCode)

	info, err := s.flightService.GetAirportInfo(airportCode)
	if err != nil {
		if errors.Is(err, flights.ErrAirportNotFound) {
			return mcp.NewToolResultText("Airport not found: " + airportCode + ". Please check the airport code and try again."), nil
		}
		if errors.Is(err, flights.ErrInvalidAirportCode) {
			return mcp.NewToolResultError("Invalid airport code: " + airportCode + ". Airport codes must be exactly 3 letters."), nil
		}
		return mcp.NewToolResultError("Failed to get airport info: " + err.Error()), nil
	}

	formattedResult := flights.FormatAirportInfo(info)
	return mcp.NewToolResultText(formattedResult), nil
}

// handleValidateFlightParams handles the validate_flight_params tool call.
func (s *Server) handleValidateFlightParams(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fromAirport, err := request.RequireString("from_airport")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: from_airport"), nil
	}

	toAirport, err := request.RequireString("to_airport")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: to_airport"), nil
	}

	departureDate, err := request.RequireString("departure_date")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: departure_date"), nil
	}

	returnDate := request.GetString("return_date", "")
	tripType := request.GetString("trip_type", "one-way")

	log.Printf("Validation request: %s -> %s on %s", fromAirport, toAirport, departureDate)

	// Build request for validation
	validationReq := &models.FlightSearchRequest{
		FromAirport:      fromAirport,
		ToAirport:        toAirport,
		DepartureDate:    departureDate,
		ReturnDate:       returnDate,
		TripType:         models.TripType(tripType),
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	result := s.flightService.ValidateParams(validationReq)
	return mcp.NewToolResultText(result.FormatResult()), nil
}

// formatErrorResult formats an error as an MCP tool result.
func formatErrorResult(err error) *mcp.CallToolResult {
	var flightErr *flights.FlightError
	if errors.As(err, &flightErr) {
		switch {
		case errors.Is(err, flights.ErrValidation):
			return mcp.NewToolResultError("**Validation Error:** " + flightErr.Details)
		case errors.Is(err, flights.ErrRateLimit):
			return mcp.NewToolResultError("**Rate Limit Error:** " + flightErr.Details + ". Please try again later.")
		case errors.Is(err, flights.ErrScrapingBlocked):
			return mcp.NewToolResultError("**Access Blocked:** " + flightErr.Details + ". Google Flights may be blocking automated access.")
		case errors.Is(err, flights.ErrScrapingFailed):
			return mcp.NewToolResultError("**Scraping Error:** " + flightErr.Details)
		case errors.Is(err, flights.ErrTimeout):
			return mcp.NewToolResultError("**Timeout Error:** The request took too long. Please try again.")
		default:
			return mcp.NewToolResultError("**Search Error:** " + flightErr.Details)
		}
	}

	return mcp.NewToolResultError("**Error:** " + err.Error())
}
