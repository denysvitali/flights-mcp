# flights-mcp

[![CI](https://github.com/denysvitali/flights-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/denysvitali/flights-mcp/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/denysvitali/flights-mcp.svg)](https://pkg.go.dev/github.com/denysvitali/flights-mcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server that lets
LLMs like Claude search Google Flights. Single Go binary, no browser, no API key.

```
> search_flights from JFK to LAX on 2026-12-15

208 flights found — cheapest $178
Best: Alaska 7:00 AM -> 2:56 PM, 1 stop, $178
      JetBlue 1:45 PM -> 10:38 PM, 1 stop, $178
      ...
```

## How it works

Google Flights encodes searches in a `tfs=` URL parameter — a base64url
protobuf message describing the itinerary. This server builds that message
with a small hand-rolled protobuf encoder (schema reverse-engineered by
[fast-flights](https://github.com/AWeirdDev/flights)), fetches the
server-rendered results page over plain HTTPS with a consent cookie, and
parses the HTML with goquery. No headless browser involved.

## Installation

```bash
# With Go
go install github.com/denysvitali/flights-mcp/cmd/flights-mcp@latest

# Or from source
git clone https://github.com/denysvitali/flights-mcp.git
cd flights-mcp
make build   # binary in bin/flights-mcp
```

Prebuilt binaries are on the [Releases](https://github.com/denysvitali/flights-mcp/releases)
page. Building needs Go 1.25+; running needs nothing else — the airport
database is embedded in the binary.

## Usage

### With Claude Code

```bash
claude mcp add flights -- flights-mcp run
```

### With Claude Desktop

Add to `~/.config/claude/claude_desktop_config.json`:

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

### With Docker

```bash
make docker-build
docker run -i --rm flights-mcp:latest run
```

### CLI

The binary doubles as a CLI for local testing:

```bash
flights-mcp info                            # server info + tool list
flights-mcp airport JFK                     # airport lookup
flights-mcp validate JFK LAX 2026-12-15     # parameter validation
flights-mcp test JFK LAX 2026-12-15         # run a real search
flights-mcp test ZRH LIS 2026-10-05 2026-10-12   # round-trip
flights-mcp version
```

## MCP Tools

### `search_flights`

Search for flights between airports. Returns flight options with prices,
times, airlines, duration, and stops, sorted by price.

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
| `passengers_infants_in_seat` | No | Infants in seat (default: 0) |
| `passengers_infants_on_lap` | No | Infants on lap (default: 0) |

### `get_airport_info`

Get information about an airport by IATA code.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `airport_code` | Yes | Airport IATA code (e.g., 'JFK', 'LAX') |

### `validate_flight_params`

Validate search parameters (airport codes, dates, passenger counts)
before making a search.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `from_airport` | Yes | Origin airport code |
| `to_airport` | Yes | Destination airport code |
| `departure_date` | Yes | Departure date |
| `return_date` | No | Return date |
| `trip_type` | No | Trip type |

## Configuration

All configuration is optional and done via environment variables
(see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `REQUEST_TIMEOUT` | `30s` | HTTP timeout per search request |
| `MAX_RETRIES` | `3` | Search retry attempts |
| `RETRY_DELAY` | `2s` | Delay between retries |
| `RATE_LIMIT_REQUESTS` | `60` | Max searches per window |
| `RATE_LIMIT_WINDOW` | `60s` | Rate limit window |
| `PROXY_URL` | – | HTTP proxy for outgoing requests |
| `AIRPORTS_FILE` | – | Override the embedded airport database |
| `LOG_LEVEL` | `info` | Log verbosity |

## Development

```bash
make test      # go test -race ./...
make lint      # golangci-lint
make build     # build for current platform
make help      # all targets
```

Project layout:

```
cmd/flights-mcp/    CLI entry point (cobra)
internal/mcp/       MCP server and tool handlers
internal/scraper/   tfs protobuf encoder + HTTP scraper + HTML parser
internal/flights/   business logic, validation, rate limiting
internal/airports/  embedded airport database
pkg/models/         data models
```

## Caveats

This scrapes Google Flights, which is an unofficial, undocumented
interface:

- The HTML markup is Google-internal and can change without notice,
  breaking the parser.
- Google may rate-limit or block automated access; the built-in rate
  limiter, user-agent rotation, and `PROXY_URL` help but are no guarantee.
- Prices are informational — always verify before booking.

For production workloads, consider an official flight API
(Amadeus, Skyscanner, Duffel).

## License

MIT — see [LICENSE](LICENSE).

## Acknowledgments

- [fast-flights](https://github.com/AWeirdDev/flights) — reverse-engineered `tfs` protobuf schema
- [mcp-go](https://github.com/mark3labs/mcp-go) — MCP SDK for Go
- [goquery](https://github.com/PuerkitoBio/goquery) — HTML parsing
- [cobra](https://github.com/spf13/cobra) — CLI framework
