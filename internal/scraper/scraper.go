// Package scraper provides the interface and implementations for scraping flight data.
package scraper

import (
	"context"

	"github.com/dvitali/flights-mcp/pkg/models"
)

// ScrapeResult contains the results of a scraping operation.
type ScrapeResult struct {
	Flights       []*models.FlightResult
	CheapestPrice string
	RawHTML       string // For debugging
}

// Scraper defines the interface for flight data scraping.
type Scraper interface {
	// SearchFlights searches for flights matching the given request.
	SearchFlights(ctx context.Context, req *models.FlightSearchRequest) (*ScrapeResult, error)

	// Close releases any resources held by the scraper.
	Close() error
}
