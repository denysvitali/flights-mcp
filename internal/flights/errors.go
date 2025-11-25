// Package flights provides the business logic for flight search operations.
package flights

import (
	"errors"
	"fmt"
)

// Sentinel errors for the flights package.
var (
	ErrValidation         = errors.New("validation error")
	ErrInvalidAirportCode = errors.New("invalid airport code")
	ErrAirportNotFound    = errors.New("airport not found")
	ErrFlightSearch       = errors.New("flight search failed")
	ErrRateLimit          = errors.New("rate limit exceeded")
	ErrScrapingBlocked    = errors.New("scraping blocked by anti-bot")
	ErrScrapingFailed     = errors.New("scraping failed")
	ErrTimeout            = errors.New("request timeout")
	ErrNoResults          = errors.New("no flight results found")
)

// FlightError provides detailed error information.
type FlightError struct {
	Op      string // Operation that failed
	Err     error  // Underlying error
	Details string // Additional context
}

// Error implements the error interface.
func (e *FlightError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Op, e.Err, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *FlightError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a new validation error.
func NewValidationError(details string) *FlightError {
	return &FlightError{
		Op:      "validate",
		Err:     ErrValidation,
		Details: details,
	}
}

// NewAirportCodeError creates a new invalid airport code error.
func NewAirportCodeError(code string) *FlightError {
	return &FlightError{
		Op:      "validate_airport",
		Err:     ErrInvalidAirportCode,
		Details: fmt.Sprintf("invalid airport code: %s", code),
	}
}

// NewAirportNotFoundError creates a new airport not found error.
func NewAirportNotFoundError(code string) *FlightError {
	return &FlightError{
		Op:      "lookup_airport",
		Err:     ErrAirportNotFound,
		Details: fmt.Sprintf("airport not found: %s", code),
	}
}

// NewSearchError creates a new flight search error.
func NewSearchError(err error) *FlightError {
	return &FlightError{
		Op:      "search",
		Err:     ErrFlightSearch,
		Details: err.Error(),
	}
}

// NewRateLimitError creates a new rate limit error.
func NewRateLimitError(limit int, window string) *FlightError {
	return &FlightError{
		Op:      "rate_limit",
		Err:     ErrRateLimit,
		Details: fmt.Sprintf("%d requests per %s", limit, window),
	}
}

// NewScrapingBlockedError creates a new scraping blocked error.
func NewScrapingBlockedError(details string) *FlightError {
	return &FlightError{
		Op:      "scrape",
		Err:     ErrScrapingBlocked,
		Details: details,
	}
}

// NewScrapingFailedError creates a new scraping failed error.
func NewScrapingFailedError(err error) *FlightError {
	return &FlightError{
		Op:      "scrape",
		Err:     ErrScrapingFailed,
		Details: err.Error(),
	}
}

// NewTimeoutError creates a new timeout error.
func NewTimeoutError(operation string) *FlightError {
	return &FlightError{
		Op:      operation,
		Err:     ErrTimeout,
		Details: "operation timed out",
	}
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrValidation) || errors.Is(err, ErrInvalidAirportCode)
}

// IsRateLimitError checks if an error is a rate limit error.
func IsRateLimitError(err error) bool {
	return errors.Is(err, ErrRateLimit)
}

// IsScrapingError checks if an error is a scraping-related error.
func IsScrapingError(err error) bool {
	return errors.Is(err, ErrScrapingBlocked) || errors.Is(err, ErrScrapingFailed)
}
