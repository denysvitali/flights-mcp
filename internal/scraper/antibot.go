package scraper

import (
	"math/rand"
	"time"
)

// AntiBotConfig contains configuration for anti-bot evasion.
type AntiBotConfig struct {
	UserAgents          []string
	EnableRandomDelays  bool
	MinDelay            time.Duration
	MaxDelay            time.Duration
	HandleCookieConsent bool
	DisableAutomation   bool
	HeadlessMode        bool
	ProxyURL            string
}

// DefaultAntiBotConfig returns the default anti-bot configuration.
func DefaultAntiBotConfig() *AntiBotConfig {
	return &AntiBotConfig{
		UserAgents:          defaultUserAgents,
		EnableRandomDelays:  true,
		MinDelay:            1 * time.Second,
		MaxDelay:            3 * time.Second,
		HandleCookieConsent: true,
		DisableAutomation:   true,
		HeadlessMode:        true,
		ProxyURL:            "",
	}
}

// RandomUserAgent returns a random user agent from the configured list.
func (c *AntiBotConfig) RandomUserAgent() string {
	if len(c.UserAgents) == 0 {
		return defaultUserAgents[0]
	}
	return c.UserAgents[rand.Intn(len(c.UserAgents))]
}

// RandomDelay returns a random duration between MinDelay and MaxDelay.
func (c *AntiBotConfig) RandomDelay() time.Duration {
	if !c.EnableRandomDelays || c.MaxDelay <= c.MinDelay {
		return c.MinDelay
	}

	delta := c.MaxDelay - c.MinDelay
	return c.MinDelay + time.Duration(rand.Int63n(int64(delta)))
}

// defaultUserAgents contains realistic browser user agents.
var defaultUserAgents = []string{
	// Chrome on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",

	// Chrome on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",

	// Chrome on Linux
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",

	// Firefox on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",

	// Firefox on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",

	// Safari on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",

	// Edge on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
}

// Cookie consent button selectors for various consent dialogs.
var cookieConsentSelectors = []string{
	// Google consent dialog (EU/GDPR) - most common patterns
	`button[jsname="b3VHJd"]`,           // Google's "Accept all" button jsname
	`button[jsname="higCR"]`,            // Alternative Google consent button
	`form[action*="consent.google.com"] button`,
	`[data-ved] button[jsaction*="click"]`,

	// Google consent by aria-label
	`button[aria-label*="Accept all"]`,
	`button[aria-label*="Accept"]`,
	`button[aria-label*="accept"]`,

	// Generic consent buttons by ID/class
	`#L2AGLb`,                           // Common Google Accept button ID
	`#W0wltc`,                           // Another Google consent button ID
	`.QS5gu`,                            // Google consent button class

	// GDPR consent frameworks
	`#onetrust-accept-btn-handler`,
	`.accept-cookies`,
	`[data-testid="cookie-policy-dialog-accept-button"]`,
}

// GetCookieConsentSelectors returns the list of cookie consent button selectors.
func GetCookieConsentSelectors() []string {
	return cookieConsentSelectors
}
