// Command flights-mcp is a Model Context Protocol server exposing Google
// Flights search as MCP tools, with a small CLI for local testing.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/denysvitali/flights-mcp/internal/airports"
	"github.com/denysvitali/flights-mcp/internal/config"
	"github.com/denysvitali/flights-mcp/internal/flights"
	mcpserver "github.com/denysvitali/flights-mcp/internal/mcp"
	"github.com/denysvitali/flights-mcp/internal/scraper"
	"github.com/denysvitali/flights-mcp/pkg/models"
)

// Set via -ldflags at build time (see Makefile).
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "flights-mcp",
		Short:         "MCP server for Google Flights search",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newRunCmd(),
		newInfoCmd(),
		newAirportCmd(),
		newValidateCmd(),
		newTestCmd(),
		newVersionCmd(),
	)

	return root
}

// loadAirports loads the airport database, preferring an explicit file over
// the embedded copy.
func loadAirports(cfg *config.Config) (*airports.Database, error) {
	db := airports.NewDatabase()
	if cfg.AirportsFile != "" {
		if err := db.LoadFromFile(cfg.AirportsFile); err != nil {
			return nil, err
		}
		return db, nil
	}
	if err := db.LoadEmbedded(); err != nil {
		return nil, err
	}
	return db, nil
}

// newScraper builds the browser-less HTTP scraper from the application config.
func newScraper(cfg *config.Config) scraper.Scraper {
	antiBot := scraper.DefaultAntiBotConfig()
	antiBot.ProxyURL = cfg.ProxyURL
	return scraper.NewHTTPScraper(antiBot, cfg.RequestTimeout)
}

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start the MCP server (stdio transport)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := loadAirports(cfg)
			if err != nil {
				return fmt.Errorf("loading airports: %w", err)
			}

			scr := newScraper(cfg)
			defer func() { _ = scr.Close() }()

			svc := flights.NewService(scr, db, cfg)
			srv := mcpserver.NewServer(cfg.ServerName, cfg.ServerVersion, svc)

			return server.ServeStdio(srv.MCPServer())
		},
	}
}

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show server information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := loadAirports(cfg)
			if err != nil {
				return fmt.Errorf("loading airports: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s %s (built %s)\n\n", cfg.ServerName, cfg.ServerVersion, buildTime)
			fmt.Fprintln(out, "MCP tools:")
			fmt.Fprintln(out, "  search_flights          Search for flights via Google Flights")
			fmt.Fprintln(out, "  get_airport_info        Look up an airport by IATA code")
			fmt.Fprintln(out, "  validate_flight_params  Validate search parameters")
			fmt.Fprintf(out, "\nAirports in database: %d\n", db.Count())
			fmt.Fprintf(out, "Transport: stdio (start with 'flights-mcp run')\n")
			return nil
		},
	}
}

func newAirportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "airport CODE",
		Short: "Show information about an airport",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := loadAirports(cfg)
			if err != nil {
				return fmt.Errorf("loading airports: %w", err)
			}

			svc := flights.NewService(nil, db, cfg)
			info, err := svc.GetAirportInfo(args[0])
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), flights.FormatAirportInfo(info))
			return nil
		},
	}
}

// buildSearchRequest builds a FlightSearchRequest from CLI positional args
// (FROM TO DEPARTURE_DATE [RETURN_DATE]).
func buildSearchRequest(args []string) *models.FlightSearchRequest {
	req := &models.FlightSearchRequest{
		FromAirport:      args[0],
		ToAirport:        args[1],
		DepartureDate:    args[2],
		TripType:         models.TripTypeOneWay,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}
	if len(args) > 3 {
		req.ReturnDate = args[3]
		req.TripType = models.TripTypeRoundTrip
	}
	return req
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate FROM TO DEPARTURE_DATE [RETURN_DATE]",
		Short: "Validate flight search parameters",
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := loadAirports(cfg)
			if err != nil {
				return fmt.Errorf("loading airports: %w", err)
			}

			svc := flights.NewService(nil, db, cfg)
			result := svc.ValidateParams(buildSearchRequest(args))

			fmt.Fprintln(cmd.OutOrStdout(), result.FormatResult())
			if !result.Valid {
				return fmt.Errorf("validation failed")
			}
			return nil
		},
	}
}

func newTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test FROM TO DEPARTURE_DATE [RETURN_DATE]",
		Short: "Run a flight search directly",
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := loadAirports(cfg)
			if err != nil {
				return fmt.Errorf("loading airports: %w", err)
			}

			scr := newScraper(cfg)
			defer func() { _ = scr.Close() }()

			svc := flights.NewService(scr, db, cfg)

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			response, err := svc.SearchFlights(ctx, buildSearchRequest(args))
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), flights.FormatFlightResults(response))
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "flights-mcp %s (built %s)\n", version, buildTime)
		},
	}
}
