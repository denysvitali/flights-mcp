package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/denysvitali/flights-mcp/pkg/models"
)

// HTTPScraper implements the Scraper interface with plain HTTP requests —
// no browser required. It builds the structured tfs search URL and parses
// the server-rendered HTML that Google Flights returns to first-time
// visitors carrying a consent cookie.
type HTTPScraper struct {
	client   *http.Client
	config   *AntiBotConfig
	currency string
}

// NewHTTPScraper creates a new browser-less scraper.
func NewHTTPScraper(config *AntiBotConfig, timeout time.Duration) *HTTPScraper {
	if config == nil {
		config = DefaultAntiBotConfig()
	}
	return &HTTPScraper{
		client:   &http.Client{Timeout: timeout},
		config:   config,
		currency: "USD",
	}
}

// SearchFlights fetches and parses Google Flights results over plain HTTP.
func (s *HTTPScraper) SearchFlights(ctx context.Context, req *models.FlightSearchRequest) (*ScrapeResult, error) {
	searchURL := BuildTFSURL(req, s.currency)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	httpReq.Header.Set("User-Agent", s.config.RandomUserAgent())
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// Pre-set consent cookies so Google serves results instead of the
	// consent interstitial (same values fast-flights uses).
	httpReq.AddCookie(&http.Cookie{Name: "CONSENT", Value: "PENDING+987"})
	httpReq.AddCookie(&http.Cookie{
		Name:  "SOCS",
		Value: "CAESHAgBEhJnd3NfMjAyMzA4MTAtMF9SQzIaAmRlIAEaBgiA_LyaBg",
	})

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fetching results: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from Google Flights", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	result, err := ParseFlightsHTML(body)
	if err != nil {
		return nil, err
	}
	if len(result.Flights) == 0 {
		if isConsentPage(body) {
			return nil, fmt.Errorf("blocked by Google consent page")
		}
		return nil, fmt.Errorf("no flights found in response (page layout may have changed)")
	}
	return result, nil
}

// Close is a no-op for the HTTP scraper.
func (s *HTTPScraper) Close() error {
	return nil
}

// isConsentPage detects the Google consent interstitial.
func isConsentPage(body []byte) bool {
	return strings.Contains(string(body), "consent.google.com")
}

// stopsRegex extracts the stop count from strings like "1 stop" / "2 stops".
var stopsRegex = regexp.MustCompile(`^(\d+) stop`)

// ParseFlightsHTML parses the server-rendered Google Flights results page.
func ParseFlightsHTML(body []byte) (*ScrapeResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	result := &ScrapeResult{}

	// Results are grouped in two containers: "best" flights and the rest.
	doc.Find(`div[jsname="IWWDBc"], div[jsname="YdtKid"]`).Each(func(groupIdx int, group *goquery.Selection) {
		isBest := group.AttrOr("jsname", "") == "IWWDBc"

		group.Find("ul.Rk10dc li").Each(func(i int, item *goquery.Selection) {
			flight := parseFlightItem(item, isBest)
			if flight != nil {
				result.Flights = append(result.Flights, flight)
			}
		})
	})

	// The current lowest price is announced in a dedicated aria-label.
	if label, ok := doc.Find("span[aria-label^='Cheapest']").Attr("aria-label"); ok {
		result.CheapestPrice = label
	}

	return result, nil
}

// parseFlightItem parses one <li> flight entry.
func parseFlightItem(item *goquery.Selection, isBest bool) *models.FlightResult {
	airline := cleanText(item.Find("div.sSHqwe.tPgKwe.ogfYpf span").First().Text())

	var depTime, arrTime string
	timeNodes := item.Find("span.mv1WYe div")
	if timeNodes.Length() >= 2 {
		depTime = cleanTimeText(timeNodes.Eq(0).Text())
		arrTime = cleanTimeText(timeNodes.Eq(1).Text())
	}

	duration := cleanText(item.Find("li div.Ak5kof div").First().Text())
	if duration == "" {
		duration = cleanText(item.Find("div.Ak5kof div").First().Text())
	}

	stopsText := cleanText(item.Find(".BbR8Ec .ogfYpf").First().Text())
	stops := parseStops(stopsText)

	price := cleanText(item.Find(".YMlIz.FpEdX").First().Text())

	// Entries without airline and price are layout artifacts (e.g. the
	// "View more flights" row), not flights.
	if airline == "" && price == "" {
		return nil
	}

	return &models.FlightResult{
		FlightName:    airline,
		Airline:       airline,
		DepartureTime: depTime,
		ArrivalTime:   arrTime,
		Duration:      duration,
		Stops:         stops,
		Price:         price,
		IsBest:        isBest,
	}
}

// parseStops converts "Nonstop" / "1 stop" / "2 stops" to a count.
// Unknown text maps to -1, which the formatter renders as "Unknown".
func parseStops(text string) int {
	if strings.EqualFold(text, "Nonstop") || strings.EqualFold(text, "Non-stop") {
		return 0
	}
	if m := stopsRegex.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return -1
}

// cleanText trims whitespace and collapses non-breaking spaces.
func cleanText(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(s)
}

// cleanTimeText strips the trailing screen-reader annotations Google adds
// to time cells (e.g. "10:30 PM on Monday" -> "10:30 PM").
func cleanTimeText(s string) string {
	s = cleanText(s)
	if idx := strings.Index(s, " on "); idx != -1 {
		s = s[:idx]
	}
	return s
}
