package scraper

import (
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/denysvitali/flights-mcp/pkg/models"
)

// The tfs URL parameter used by Google Flights is a base64url-encoded
// protobuf message describing the itinerary. The schema below is the one
// reverse-engineered by the fast-flights project:
//
//	message Airport   { string airport = 2; }
//	message FlightData {
//	    string date = 2;          // YYYY-MM-DD
//	    Airport from_flight = 13;
//	    Airport to_flight = 14;
//	}
//	message Info {
//	    repeated FlightData data = 3;
//	    repeated Passenger passengers = 8;  // enum, varint
//	    Seat seat = 9;                      // enum, varint
//	    Trip trip = 19;                     // enum, varint
//	}
//
// The messages are small enough that we encode the protobuf wire format by
// hand instead of pulling in a protobuf dependency.

// Protobuf enum values for the tfs message.
const (
	tfsSeatEconomy        = 1
	tfsSeatPremiumEconomy = 2
	tfsSeatBusiness       = 3
	tfsSeatFirst          = 4

	tfsTripRoundTrip = 1
	tfsTripOneWay    = 2

	tfsPassengerAdult        = 1
	tfsPassengerChild        = 2
	tfsPassengerInfantInSeat = 3
	tfsPassengerInfantOnLap  = 4
)

// appendVarint appends a protobuf varint.
func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// appendTag appends a protobuf field tag with the given wire type.
func appendTag(b []byte, field int, wireType int) []byte {
	return appendVarint(b, uint64(field)<<3|uint64(wireType))
}

// appendString appends a length-delimited string field.
func appendString(b []byte, field int, s string) []byte {
	b = appendTag(b, field, 2)
	b = appendVarint(b, uint64(len(s)))
	return append(b, s...)
}

// appendMessage appends a length-delimited embedded message field.
func appendMessage(b []byte, field int, msg []byte) []byte {
	b = appendTag(b, field, 2)
	b = appendVarint(b, uint64(len(msg)))
	return append(b, msg...)
}

// appendEnum appends a varint enum field.
func appendEnum(b []byte, field int, v int) []byte {
	b = appendTag(b, field, 0)
	return appendVarint(b, uint64(v))
}

// encodeAirport encodes an Airport message.
func encodeAirport(code string) []byte {
	return appendString(nil, 2, code)
}

// encodeFlightData encodes one leg of the itinerary.
func encodeFlightData(date, from, to string) []byte {
	b := appendString(nil, 2, date)
	b = appendMessage(b, 13, encodeAirport(from))
	b = appendMessage(b, 14, encodeAirport(to))
	return b
}

// seatToTFS maps a models.SeatClass to its tfs enum value.
func seatToTFS(seat models.SeatClass) int {
	switch seat {
	case models.SeatClassPremiumEconomy:
		return tfsSeatPremiumEconomy
	case models.SeatClassBusiness:
		return tfsSeatBusiness
	case models.SeatClassFirst:
		return tfsSeatFirst
	default:
		return tfsSeatEconomy
	}
}

// EncodeTFS encodes a flight search request as a base64url tfs parameter.
func EncodeTFS(req *models.FlightSearchRequest) string {
	var info []byte

	// Legs (field 3, repeated)
	info = appendMessage(info, 3, encodeFlightData(req.DepartureDate, req.FromAirport, req.ToAirport))
	if req.TripType == models.TripTypeRoundTrip && req.ReturnDate != "" {
		info = appendMessage(info, 3, encodeFlightData(req.ReturnDate, req.ToAirport, req.FromAirport))
	}

	// Passengers (field 8, repeated enum)
	adults := req.PassengersAdults
	if adults <= 0 {
		adults = 1
	}
	for i := 0; i < adults; i++ {
		info = appendEnum(info, 8, tfsPassengerAdult)
	}
	for i := 0; i < req.PassengersChildren; i++ {
		info = appendEnum(info, 8, tfsPassengerChild)
	}
	for i := 0; i < req.PassengersInfantsSeat; i++ {
		info = appendEnum(info, 8, tfsPassengerInfantInSeat)
	}
	for i := 0; i < req.PassengersInfantsLap; i++ {
		info = appendEnum(info, 8, tfsPassengerInfantOnLap)
	}

	// Seat class (field 9)
	info = appendEnum(info, 9, seatToTFS(req.SeatClass))

	// Trip type (field 19)
	trip := tfsTripOneWay
	if req.TripType == models.TripTypeRoundTrip && req.ReturnDate != "" {
		trip = tfsTripRoundTrip
	}
	info = appendEnum(info, 19, trip)

	return base64.URLEncoding.EncodeToString(info)
}

// BuildTFSURL builds a Google Flights search URL from a request using the
// structured tfs parameter. Unlike the free-text query URLs, this lands
// directly on fully-specified search results.
func BuildTFSURL(req *models.FlightSearchRequest, currency string) string {
	params := url.Values{}
	params.Set("tfs", EncodeTFS(req))
	params.Set("hl", "en")
	params.Set("tfu", "EgQIABABIgA")
	if currency != "" {
		params.Set("curr", currency)
	}
	return fmt.Sprintf("https://www.google.com/travel/flights?%s", params.Encode())
}
