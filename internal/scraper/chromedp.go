package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/dvitali/flights-mcp/pkg/models"
)

// ChromeDPScraper implements the Scraper interface using chromedp.
type ChromeDPScraper struct {
	config    *AntiBotConfig
	allocCtx  context.Context
	cancel    context.CancelFunc
	parser    *Parser
	timeout   time.Duration
}

// NewChromeDPScraper creates a new ChromeDPScraper.
func NewChromeDPScraper(config *AntiBotConfig, timeout time.Duration) (*ChromeDPScraper, error) {
	if config == nil {
		config = DefaultAntiBotConfig()
	}

	// Build chromedp options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// Anti-bot measures
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),

		// Set user agent
		chromedp.UserAgent(config.RandomUserAgent()),

		// Window size for realistic viewport
		chromedp.WindowSize(1920, 1080),
	)

	// Add headless mode if configured
	if config.HeadlessMode {
		opts = append(opts, chromedp.Flag("headless", true))
	} else {
		opts = append(opts, chromedp.Flag("headless", false))
	}

	// Add proxy if configured
	if config.ProxyURL != "" {
		opts = append(opts, chromedp.ProxyServer(config.ProxyURL))
	}

	// Create allocator context
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)

	return &ChromeDPScraper{
		config:   config,
		allocCtx: allocCtx,
		cancel:   cancel,
		parser:   NewParser(),
		timeout:  timeout,
	}, nil
}

// SearchFlights searches for flights using Google Flights.
func (s *ChromeDPScraper) SearchFlights(ctx context.Context, req *models.FlightSearchRequest) (*ScrapeResult, error) {
	// Create browser context from allocator
	browserCtx, browserCancel := chromedp.NewContext(s.allocCtx)
	defer browserCancel()

	// Apply timeout to the browser context
	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, s.timeout)
	defer timeoutCancel()

	// Storage for captured API responses
	var apiResponses [][]byte
	var apiMu sync.Mutex
	var captureWg sync.WaitGroup

	// Track responses and their bodies via LoadingFinished event
	responseURLs := make(map[network.RequestID]string)
	var responseMu sync.Mutex

	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			url := e.Response.URL
			// Track URLs we're interested in
			if strings.Contains(url, "FlightsFrontendService") ||
			   strings.Contains(url, "batchexecute") {
				responseMu.Lock()
				responseURLs[e.RequestID] = url
				responseMu.Unlock()
				log.Printf("Tracking response: %s", url[:min(80, len(url))])
			}

		case *network.EventLoadingFinished:
			// Response is now complete, safe to get body
			responseMu.Lock()
			url, tracked := responseURLs[e.RequestID]
			delete(responseURLs, e.RequestID)
			responseMu.Unlock()

			if tracked {
				captureWg.Add(1)
				go func(requestID network.RequestID, capturedURL string) {
					defer captureWg.Done()
					var body []byte
					err := chromedp.Run(browserCtx,
						chromedp.ActionFunc(func(ctx context.Context) error {
							var err error
							body, err = network.GetResponseBody(requestID).Do(ctx)
							return err
						}),
					)
					if err == nil && len(body) > 100 {
						apiMu.Lock()
						apiResponses = append(apiResponses, body)
						log.Printf("*** CAPTURED: %d bytes from %s", len(body), capturedURL[:min(60, len(capturedURL))])
						apiMu.Unlock()
					} else if err != nil {
						log.Printf("Failed to get response body: %v", err)
					}
				}(e.RequestID, url)
			}
		}
	})

	// Build the search URL
	url := BuildGoogleFlightsURL(req)
	log.Printf("Navigating to: %s", url)

	var htmlContent string

	// Execute the scraping workflow with logging
	log.Println("Starting browser...")
	err := chromedp.Run(browserCtx,
		// Enable network tracking
		network.Enable(),

		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Browser started, navigating...")
			return nil
		}),

		// Navigate to Google Flights
		chromedp.Navigate(url),

		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Page loaded, waiting for initial render...")
			return nil
		}),

		// Wait for initial page load
		chromedp.Sleep(2*time.Second),

		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Checking for cookie consent...")
			return nil
		}),

		// Handle cookie consent if needed
		s.handleCookieConsent(),

		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Waiting for flight results...")
			return nil
		}),

		// Wait for flight results to load
		s.waitForResults(),

		// Extra wait for API calls to complete
		chromedp.Sleep(2*time.Second),

		// Random delay to appear more human
		s.randomDelay(),

		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Scrolling page...")
			return nil
		}),

		// Scroll to load more results
		s.scrollPage(),

		// Wait for more data to load after scroll
		chromedp.Sleep(1*time.Second),

		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Extracting content...")
			return nil
		}),

		// Extract the HTML content as fallback
		chromedp.OuterHTML("body", &htmlContent, chromedp.ByQuery),
	)

	if err != nil {
		return nil, fmt.Errorf("scraping failed: %w", err)
	}

	// Wait for all capture goroutines to complete
	log.Println("Waiting for API captures to complete...")
	captureWg.Wait()

	// Log captured responses
	apiMu.Lock()
	capturedCount := len(apiResponses)
	apiMu.Unlock()
	log.Printf("Captured %d API responses", capturedCount)

	// Save API responses for debugging
	for i, resp := range apiResponses {
		debugFile := fmt.Sprintf("/tmp/flights_api_%d.json", i)
		os.WriteFile(debugFile, resp, 0644)
		log.Printf("Saved API response to %s", debugFile)
	}

	// Try to parse API responses first
	result := s.parseAPIResponses(apiResponses)
	if result != nil && len(result.Flights) > 0 {
		log.Printf("Parsed %d flights from API responses", len(result.Flights))
		return result, nil
	}

	// Fallback to HTML parsing
	log.Println("Falling back to HTML parsing...")
	log.Printf("Extracted HTML: %d bytes", len(htmlContent))

	// Save HTML to file for debugging
	if len(htmlContent) > 0 {
		debugFile := "/tmp/flights_debug.html"
		if err := os.WriteFile(debugFile, []byte(htmlContent), 0644); err == nil {
			log.Printf("Saved HTML to %s for debugging", debugFile)
		}
	}

	// Parse the HTML to extract flight data
	result, err = s.parser.Parse(htmlContent)
	if err != nil {
		return nil, fmt.Errorf("parsing failed: %w", err)
	}

	result.RawHTML = htmlContent
	return result, nil
}

// parseAPIResponses attempts to parse flight data from captured API responses.
func (s *ChromeDPScraper) parseAPIResponses(responses [][]byte) *ScrapeResult {
	var flights []*models.FlightResult

	for _, resp := range responses {
		// Use the structured flight parser
		parsed := ParseFlightsFromResponse(resp)
		if len(parsed) > 0 {
			log.Printf("Parsed %d flights from API response", len(parsed))
			flights = append(flights, parsed...)
		}
	}

	if len(flights) == 0 {
		return nil
	}

	// Find cheapest
	var cheapest string
	var cheapestPrice float64 = -1
	for _, f := range flights {
		if price := parseFlightPrice(f.Price); price > 0 {
			if cheapestPrice < 0 || price < cheapestPrice {
				cheapestPrice = price
				cheapest = f.Price
			}
		}
	}

	return &ScrapeResult{
		Flights:       flights,
		CheapestPrice: cheapest,
	}
}

// extractFlightsFromJSON tries to extract flight data from a JSON response.
func (s *ChromeDPScraper) extractFlightsFromJSON(data []byte) []*models.FlightResult {
	var flights []*models.FlightResult

	// Google's batchexecute responses have a specific format
	// They often start with )]}'  followed by JSON
	dataStr := string(data)
	if strings.HasPrefix(dataStr, ")]}'") {
		dataStr = dataStr[4:]
		data = []byte(dataStr)
	}

	// Try to parse as JSON array
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		// Not valid JSON, try to extract embedded JSON
		flights = s.extractFlightsFromText(dataStr)
		return flights
	}

	// Recursively search for flight data in the JSON structure
	flights = s.searchJSONForFlights(jsonData)
	return flights
}

// searchJSONForFlights recursively searches JSON for flight data.
func (s *ChromeDPScraper) searchJSONForFlights(data interface{}) []*models.FlightResult {
	var flights []*models.FlightResult

	switch v := data.(type) {
	case []interface{}:
		// Check if this array looks like flight data
		if flight := s.tryParseAsFlightArray(v); flight != nil {
			flights = append(flights, flight)
		}
		// Recurse into array elements
		for _, item := range v {
			flights = append(flights, s.searchJSONForFlights(item)...)
		}
	case map[string]interface{}:
		// Recurse into map values
		for _, val := range v {
			flights = append(flights, s.searchJSONForFlights(val)...)
		}
	}

	return flights
}

// tryParseAsFlightArray checks if an array contains flight data.
func (s *ChromeDPScraper) tryParseAsFlightArray(arr []interface{}) *models.FlightResult {
	// Google's flight data often has specific patterns
	// Look for arrays that contain price, airline, times, etc.

	// This is heuristic - look for arrays with expected flight data patterns
	hasPrice := false
	hasAirline := false

	for _, item := range arr {
		switch v := item.(type) {
		case float64:
			// Could be a price if it's in reasonable range
			if v > 50 && v < 50000 {
				hasPrice = true
			}
		case string:
			// Could be airline name
			airlines := []string{"United", "Delta", "American", "JetBlue", "Southwest", "Frontier", "Spirit", "Alaska"}
			for _, airline := range airlines {
				if strings.Contains(v, airline) {
					hasAirline = true
					break
				}
			}
		}
	}

	// TODO: Implement more sophisticated parsing based on actual API structure
	// For now, return nil - we'll rely on HTML parsing
	_ = hasPrice
	_ = hasAirline

	return nil
}

// extractFlightsFromText extracts flight data from text that may contain embedded JSON.
func (s *ChromeDPScraper) extractFlightsFromText(text string) []*models.FlightResult {
	// Look for price patterns and airline names in the text
	// This is a fallback when JSON parsing fails
	return nil
}

// parseFlightPrice extracts a numeric price from a price string.
func parseFlightPrice(priceStr string) float64 {
	// Remove currency symbols and parse
	cleaned := strings.ReplaceAll(priceStr, "$", "")
	cleaned = strings.ReplaceAll(cleaned, "€", "")
	cleaned = strings.ReplaceAll(cleaned, "£", "")
	cleaned = strings.ReplaceAll(cleaned, "CHF", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSpace(cleaned)

	var price float64
	fmt.Sscanf(cleaned, "%f", &price)
	return price
}

// Close releases resources.
func (s *ChromeDPScraper) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// handleCookieConsent attempts to click cookie consent buttons.
func (s *ChromeDPScraper) handleCookieConsent() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		if !s.config.HandleCookieConsent {
			log.Println("Cookie consent handling disabled")
			return nil
		}

		// Try each cookie consent selector with a short timeout
		for _, selector := range GetCookieConsentSelectors() {
			// Create a short timeout context for each selector check
			checkCtx, cancel := context.WithTimeout(ctx, 1*time.Second)

			var nodes []*cdp.Node
			err := chromedp.Nodes(selector, &nodes, chromedp.AtLeast(0)).Do(checkCtx)
			cancel()

			if err == nil && len(nodes) > 0 {
				log.Printf("Found cookie consent button: %s (count: %d)", selector, len(nodes))

				// Use JavaScript click which is more reliable
				jsClick := fmt.Sprintf(`document.querySelector('%s').click()`, selector)
				clickCtx, clickCancel := context.WithTimeout(ctx, 2*time.Second)
				err = chromedp.Evaluate(jsClick, nil).Do(clickCtx)
				clickCancel()

				if err == nil {
					log.Printf("Clicked cookie consent button via JS: %s", selector)
					// Wait for consent to be processed
					time.Sleep(1 * time.Second)
					return nil
				}
				log.Printf("JS click failed, trying direct click: %v", err)

				// Fallback to direct click
				clickCtx2, clickCancel2 := context.WithTimeout(ctx, 2*time.Second)
				err = chromedp.Click(selector, chromedp.NodeVisible).Do(clickCtx2)
				clickCancel2()

				if err == nil {
					log.Printf("Clicked cookie consent button: %s", selector)
					time.Sleep(1 * time.Second)
					return nil
				}
				log.Printf("Direct click also failed: %v", err)
			}
		}

		log.Println("No cookie consent dialog found or could not click")
		return nil
	}
}

// waitForResults waits for flight results to appear on the page.
func (s *ChromeDPScraper) waitForResults() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		// Try multiple selectors for flight results with timeouts
		selectors := []string{
			`[data-result-key]`,
			`[jsname="IWWDBc"]`,
			`.gws-flights-results__result-item`,
			`[data-ved]`,
			`ul li`,  // Generic list items as fallback
		}

		for _, selector := range selectors {
			log.Printf("Checking for selector: %s", selector)
			// Use a short timeout per selector
			checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := chromedp.WaitVisible(selector, chromedp.ByQuery).Do(checkCtx)
			cancel()

			if err == nil {
				log.Printf("Found results with selector: %s", selector)
				return nil
			}
		}

		// If no results found, wait a bit longer and continue anyway
		log.Println("No specific result selectors found, continuing anyway...")
		return chromedp.Sleep(2 * time.Second).Do(ctx)
	}
}

// randomDelay adds a random delay to appear more human.
func (s *ChromeDPScraper) randomDelay() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		if s.config.EnableRandomDelays {
			delay := s.config.RandomDelay()
			log.Printf("Random delay: %v", delay)
			time.Sleep(delay)
		}
		return nil
	}
}

// scrollPage scrolls the page to load more results.
func (s *ChromeDPScraper) scrollPage() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		// Scroll down to trigger lazy loading
		err := chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight / 2)`, nil).Do(ctx)
		if err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)

		err = chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil).Do(ctx)
		if err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)

		// Scroll back up
		err = chromedp.Evaluate(`window.scrollTo(0, 0)`, nil).Do(ctx)
		return err
	}
}
