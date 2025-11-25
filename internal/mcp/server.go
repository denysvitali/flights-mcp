// Package mcp provides the Model Context Protocol server implementation.
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/dvitali/flights-mcp/internal/flights"
)

// Server wraps the MCP server with flight service functionality.
type Server struct {
	mcpServer     *server.MCPServer
	flightService *flights.Service
}

// NewServer creates a new MCP server with flight search capabilities.
func NewServer(name, version string, flightService *flights.Service) *Server {
	s := &Server{
		flightService: flightService,
	}

	// Create MCP server
	s.mcpServer = server.NewMCPServer(
		name,
		version,
		server.WithToolCapabilities(true),
	)

	// Register tools
	s.registerTools()

	return s
}

// registerTools registers all MCP tools.
func (s *Server) registerTools() {
	s.registerSearchFlightsTool()
	s.registerGetAirportInfoTool()
	s.registerValidateFlightParamsTool()
}

// registerSearchFlightsTool registers the search_flights tool.
func (s *Server) registerSearchFlightsTool() {
	tool := mcp.NewTool("search_flights",
		mcp.WithDescription("Search for flights using Google Flights. Returns flight options with prices, times, and airlines."),
		mcp.WithString("from_airport",
			mcp.Required(),
			mcp.Description("Origin airport IATA code (e.g., 'JFK', 'LAX')")),
		mcp.WithString("to_airport",
			mcp.Required(),
			mcp.Description("Destination airport IATA code (e.g., 'LAX', 'LHR')")),
		mcp.WithString("departure_date",
			mcp.Required(),
			mcp.Description("Departure date in YYYY-MM-DD format")),
		mcp.WithString("return_date",
			mcp.Description("Return date in YYYY-MM-DD format (for round-trip flights)")),
		mcp.WithString("trip_type",
			mcp.Description("Trip type: 'one-way' or 'round-trip' (default: 'one-way')")),
		mcp.WithString("seat_class",
			mcp.Description("Seat class: 'economy', 'premium-economy', 'business', or 'first' (default: 'economy')")),
		mcp.WithNumber("passengers_adults",
			mcp.Description("Number of adult passengers (default: 1)")),
		mcp.WithNumber("passengers_children",
			mcp.Description("Number of child passengers (default: 0)")),
		mcp.WithNumber("passengers_infants_in_seat",
			mcp.Description("Number of infants in seat (default: 0)")),
		mcp.WithNumber("passengers_infants_on_lap",
			mcp.Description("Number of infants on lap (default: 0)")),
	)

	s.mcpServer.AddTool(tool, s.handleSearchFlights)
}

// registerGetAirportInfoTool registers the get_airport_info tool.
func (s *Server) registerGetAirportInfoTool() {
	tool := mcp.NewTool("get_airport_info",
		mcp.WithDescription("Get information about an airport by its IATA code."),
		mcp.WithString("airport_code",
			mcp.Required(),
			mcp.Description("Airport IATA code (e.g., 'JFK', 'LAX')")),
	)

	s.mcpServer.AddTool(tool, s.handleGetAirportInfo)
}

// registerValidateFlightParamsTool registers the validate_flight_params tool.
func (s *Server) registerValidateFlightParamsTool() {
	tool := mcp.NewTool("validate_flight_params",
		mcp.WithDescription("Validate flight search parameters before making a search. Use this to check if parameters are valid."),
		mcp.WithString("from_airport",
			mcp.Required(),
			mcp.Description("Origin airport IATA code")),
		mcp.WithString("to_airport",
			mcp.Required(),
			mcp.Description("Destination airport IATA code")),
		mcp.WithString("departure_date",
			mcp.Required(),
			mcp.Description("Departure date in YYYY-MM-DD format")),
		mcp.WithString("return_date",
			mcp.Description("Return date in YYYY-MM-DD format")),
		mcp.WithString("trip_type",
			mcp.Description("Trip type: 'one-way' or 'round-trip'")),
	)

	s.mcpServer.AddTool(tool, s.handleValidateFlightParams)
}

// MCPServer returns the underlying MCP server for running.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}
