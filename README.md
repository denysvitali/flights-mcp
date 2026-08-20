# Flights MCP Server

A Model Context Protocol (MCP) server that exposes Google Flights search functionality as MCP tools. This allows LLMs like Claude to search for flights directly through a standardized protocol.

**Written in Go** for single-binary deployment, better performance, and production robustness.

## Features

- **Flight Search**: Search for flights between airports using Google Flights
- **No Browser Needed**: Plain HTTP with a protobuf-encoded search URL — no Chrome, no headless browser
- **Airport Information**: Get details about airports (50+ major airports embedded in the binary)
- **Parameter Validation**: Validate flight search parameters before making requests
- **MCP Compliant**: Built using the mcp-go SDK
- **Single Binary**: No runtime dependencies, easy deployment

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/denysvitali/flights-mcp.git
cd flights-mcp

# Build the binary
make build

# Or install to GOPATH/bin
make install
```

### From Binary

Download the latest release from the [Releases](https://github.com/denysvitali/flights-mcp/releases) page.

### Requirements

- Go 1.25+ (for building)

## Usage

### As MCP Server

Start the MCP server (connects via stdio):

```bash
flights-mcp run
```

Add to your Claude Desktop configuration (`~/.config/claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "flights": {
      "command": "/path/to/flights-mcp",
      "args": ["run"]
    }
  }
}
```

### CLI Commands

```bash
# Show server information
flights-mcp info

# Get airport information
flights-mcp airport JFK

# Validate search parameters
flights-mcp validate JFK LAX 2025-12-15

# Test flight search directly
flights-mcp test JFK LAX 2025-12-15

# Show version
flights-mcp version
```

## MCP Tools

### `search_flights`

Search for flights between airports.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `from_airport` | Yes | Origin airport IATA code (e.g., 'JFK') |
| `to_airport` | Yes | Destination airport IATA code |
| `departure_date` | Yes | Date in YYYY-MM-DD format |
| `return_date` | No | Return date for round-trip |
| `trip_type` | No | 'one-way' or 'round-trip' (default: 'one-way') |
| `seat_class` | No | 'economy', 'premium-economy', 'business', 'first' |
| `passengers_adults` | No | Number of adults (default: 1) |
| `passengers_children` | No | Number of children (default: 0) |

### `get_airport_info`

Get information about an airport by IATA code.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `airport_code` | Yes | Airport IATA code (e.g., 'JFK', 'LAX') |

### `validate_flight_params`

Validate search parameters before making a search.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `from_airport` | Yes | Origin airport code |
| `to_airport` | Yes | Destination airport code |
| `departure_date` | Yes | Departure date |
| `return_date` | No | Return date |
| `trip_type` | No | Trip type |

## Configuration

Configuration is done via environment variables:

```bash
# Server settings
SERVER_NAME=flights-mcp
SERVER_VERSION=1.0.0
LOG_LEVEL=info

# Scraper
REQUEST_TIMEOUT=30s
MAX_RETRIES=3

# Rate limiting
RATE_LIMIT_REQUESTS=60
RATE_LIMIT_WINDOW=60s

# Anti-bot
PROXY_URL=
```

Copy `.env.example` to `.env` and customize as needed.

## Project Structure

```
flights-mcp/
├── cmd/
│   └── flights-mcp/
│       └── main.go           # CLI entry point
├── internal/
│   ├── config/               # Configuration management
│   ├── mcp/                  # MCP server and tools
│   ├── scraper/              # HTTP scraper implementation
│   ├── flights/              # Business logic, validation
│   └── airports/             # Airport database
├── pkg/
│   └── models/               # Data models
├── Makefile
├── Dockerfile
└── go.mod
```

## Development

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run linter
make lint
```

### Testing

```bash
# Run all tests
go test -v ./...

# Run with coverage
make test-coverage
```

### Docker

```bash
# Build Docker image
make docker-build

# Run in Docker
docker run -it flights-mcp:latest info
```

## Known Limitations

### Google Flights Scraping

The scraper encodes the search as the same protobuf `tfs=` URL parameter
the Google Flights frontend uses, fetches the server-rendered results page
with a consent cookie, and parses the HTML — no browser involved. This is
fast and reliable today, but the page markup is Google-internal and may
change without notice.

Google Flights also uses anti-bot measures that may block scraping:
- Cookie consent walls
- Rate limiting
- Browser fingerprinting

Countermeasures included:
- Consent cookies pre-set on requests
- User agent rotation, optional proxy support

**Scraping may fail** if Google's anti-bot detection is triggered or the page layout changes. For production use, consider:
- Using a paid flight API (Amadeus, Skyscanner)
- Running with a residential proxy
- Respecting rate limits

## License

MIT License - see [LICENSE](LICENSE)

## Acknowledgments

- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP SDK for Go
- [goquery](https://github.com/PuerkitoBio/goquery) - HTML parsing
- [cobra](https://github.com/spf13/cobra) - CLI framework
- [fast-flights](https://github.com/AWeirdDev/flights) - reverse-engineered tfs protobuf schema
