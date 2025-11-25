package models

// FlightResult represents a single flight result from a search.
type FlightResult struct {
	FlightName    string `json:"flight_name"`
	Airline       string `json:"airline"`
	DepartureTime string `json:"departure_time"`
	ArrivalTime   string `json:"arrival_time"`
	Duration      string `json:"duration"`
	Stops         int    `json:"stops"`
	Price         string `json:"price"`
	IsBest        bool   `json:"is_best"`
}
