package airports

import _ "embed"

//go:embed airports.json
var embeddedAirports []byte

// LoadEmbedded loads the airport database bundled into the binary.
func (db *Database) LoadEmbedded() error {
	return db.LoadFromJSON(embeddedAirports)
}
