// Package models contains the data models for the flights-mcp server.
package models

// AirportInfo represents information about an airport.
type AirportInfo struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	City    string `json:"city"`
	Country string `json:"country"`
}
