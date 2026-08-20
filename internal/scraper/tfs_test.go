package scraper

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/flights-mcp/pkg/models"
)

func TestEncodeTFSOneWay(t *testing.T) {
	req := &models.FlightSearchRequest{
		FromAirport:      "TPE",
		ToAirport:        "MYJ",
		DepartureDate:    "2025-01-01",
		TripType:         models.TripTypeOneWay,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	// Hand-computed protobuf wire format:
	//   field 3 (FlightData): date="2025-01-01", from=TPE, to=MYJ
	//   field 8 (passenger):  ADULT (1)
	//   field 9 (seat):       ECONOMY (1)
	//   field 19 (trip):      ONE_WAY (2)
	want := []byte{
		0x1A, 0x1A, // field 3, len 26
		0x12, 0x0A, '2', '0', '2', '5', '-', '0', '1', '-', '0', '1', // date
		0x6A, 0x05, 0x12, 0x03, 'T', 'P', 'E', // from
		0x72, 0x05, 0x12, 0x03, 'M', 'Y', 'J', // to
		0x40, 0x01, // passenger: adult
		0x48, 0x01, // seat: economy
		0x98, 0x01, 0x02, // trip: one-way
	}

	got, err := base64.URLEncoding.DecodeString(EncodeTFS(req))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEncodeTFSRoundTrip(t *testing.T) {
	req := &models.FlightSearchRequest{
		FromAirport:        "JFK",
		ToAirport:          "LAX",
		DepartureDate:      "2025-12-15",
		ReturnDate:         "2025-12-22",
		TripType:           models.TripTypeRoundTrip,
		SeatClass:          models.SeatClassBusiness,
		PassengersAdults:   2,
		PassengersChildren: 1,
	}

	got, err := base64.URLEncoding.DecodeString(EncodeTFS(req))
	require.NoError(t, err)

	s := string(got)
	assert.Contains(t, s, "2025-12-15")
	assert.Contains(t, s, "2025-12-22")
	// Return leg is reversed: LAX -> JFK
	assert.Less(t, strings.Index(s, "JFK"), strings.Index(s, "LAX"))
	// Trip enum: field 19 varint = ROUND_TRIP (1)
	assert.Equal(t, []byte{0x98, 0x01, 0x01}, got[len(got)-3:])
	// Passengers: 2 adults + 1 child
	assert.Equal(t, 3, strings.Count(s, string([]byte{0x40})))
}

func TestBuildTFSURL(t *testing.T) {
	req := &models.FlightSearchRequest{
		FromAirport:      "ZRH",
		ToAirport:        "LIS",
		DepartureDate:    "2026-09-01",
		TripType:         models.TripTypeOneWay,
		SeatClass:        models.SeatClassEconomy,
		PassengersAdults: 1,
	}

	u := BuildTFSURL(req, "CHF")
	assert.True(t, strings.HasPrefix(u, "https://www.google.com/travel/flights?"))
	assert.Contains(t, u, "tfs=")
	assert.Contains(t, u, "curr=CHF")
	assert.Contains(t, u, "hl=en")
}

func TestParseStops(t *testing.T) {
	assert.Equal(t, 0, parseStops("Nonstop"))
	assert.Equal(t, 1, parseStops("1 stop"))
	assert.Equal(t, 2, parseStops("2 stops"))
	assert.Equal(t, -1, parseStops(""))
	assert.Equal(t, -1, parseStops("Separate tickets"))
}

func TestParseFlightsHTML(t *testing.T) {
	html := `<html><body>
	<div jsname="IWWDBc"><ul class="Rk10dc">
	  <li>
	    <div class="sSHqwe tPgKwe ogfYpf"><span>Swiss</span></div>
	    <span class="mv1WYe"><div>8:15 AM</div><div>11:30 AM</div></span>
	    <div class="Ak5kof"><div>2 hr 15 min</div></div>
	    <div class="BbR8Ec"><div class="ogfYpf">Nonstop</div></div>
	    <span class="YMlIz FpEdX">$142</span>
	  </li>
	</ul></div>
	<div jsname="YdtKid"><ul class="Rk10dc">
	  <li>
	    <div class="sSHqwe tPgKwe ogfYpf"><span>TAP Air Portugal</span></div>
	    <span class="mv1WYe"><div>6:40 PM</div><div>10:05 PM</div></span>
	    <div class="Ak5kof"><div>5 hr 25 min</div></div>
	    <div class="BbR8Ec"><div class="ogfYpf">1 stop</div></div>
	    <span class="YMlIz FpEdX">$98</span>
	  </li>
	  <li></li>
	</ul></div>
	</body></html>`

	result, err := ParseFlightsHTML([]byte(html))
	require.NoError(t, err)
	require.Len(t, result.Flights, 2)

	best := result.Flights[0]
	assert.Equal(t, "Swiss", best.Airline)
	assert.Equal(t, "8:15 AM", best.DepartureTime)
	assert.Equal(t, "11:30 AM", best.ArrivalTime)
	assert.Equal(t, "2 hr 15 min", best.Duration)
	assert.Equal(t, 0, best.Stops)
	assert.Equal(t, "$142", best.Price)
	assert.True(t, best.IsBest)

	other := result.Flights[1]
	assert.Equal(t, "TAP Air Portugal", other.Airline)
	assert.Equal(t, 1, other.Stops)
	assert.Equal(t, "$98", other.Price)
	assert.False(t, other.IsBest)
}
