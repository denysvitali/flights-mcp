package airports

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeAirportCode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid uppercase", "JFK", "JFK", false},
		{"valid lowercase", "jfk", "JFK", false},
		{"valid mixed case", "JfK", "JFK", false},
		{"with spaces", " JFK ", "JFK", false},
		{"with special chars", "J-F-K", "JFK", false},
		{"too short", "JF", "", true},
		{"too long", "JFKX", "", true},
		{"empty", "", "", true},
		{"numbers only", "123", "", true},
		{"with numbers", "JF1", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeAirportCode(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIsValidAirportCode(t *testing.T) {
	assert.True(t, IsValidAirportCode("JFK"))
	assert.True(t, IsValidAirportCode("lax"))
	assert.True(t, IsValidAirportCode(" SFO "))
	assert.False(t, IsValidAirportCode("JF"))
	assert.False(t, IsValidAirportCode("JFKX"))
	assert.False(t, IsValidAirportCode(""))
}

func TestDatabase_LoadFromJSON(t *testing.T) {
	db := NewDatabase()

	jsonData := `{
		"airports": [
			{"code": "JFK", "name": "John F. Kennedy", "city": "New York", "country": "USA"},
			{"code": "LAX", "name": "Los Angeles International", "city": "Los Angeles", "country": "USA"}
		]
	}`

	err := db.LoadFromJSON([]byte(jsonData))
	require.NoError(t, err)

	assert.Equal(t, 2, db.Count())

	jfk, ok := db.Get("JFK")
	require.True(t, ok)
	assert.Equal(t, "JFK", jfk.Code)
	assert.Equal(t, "John F. Kennedy", jfk.Name)
	assert.Equal(t, "New York", jfk.City)

	lax, ok := db.Get("lax")
	require.True(t, ok)
	assert.Equal(t, "LAX", lax.Code)
}

func TestDatabase_Get(t *testing.T) {
	db := NewDatabase()
	require.NoError(t, db.LoadFromJSON([]byte(`{
		"airports": [
			{"code": "JFK", "name": "JFK Airport", "city": "NYC", "country": "USA"}
		]
	}`)))

	// Valid code
	airport, ok := db.Get("JFK")
	assert.True(t, ok)
	assert.Equal(t, "JFK", airport.Code)

	// Lowercase
	airport, ok = db.Get("jfk")
	assert.True(t, ok)
	assert.Equal(t, "JFK", airport.Code)

	// Not found
	airport, ok = db.Get("XXX")
	assert.False(t, ok)
	assert.Nil(t, airport)

	// Invalid code
	airport, ok = db.Get("invalid")
	assert.False(t, ok)
	assert.Nil(t, airport)
}

func TestDatabase_Exists(t *testing.T) {
	db := NewDatabase()
	require.NoError(t, db.LoadFromJSON([]byte(`{
		"airports": [
			{"code": "JFK", "name": "JFK", "city": "NYC", "country": "USA"}
		]
	}`)))

	assert.True(t, db.Exists("JFK"))
	assert.True(t, db.Exists("jfk"))
	assert.False(t, db.Exists("XXX"))
}

func TestDatabase_All(t *testing.T) {
	db := NewDatabase()
	require.NoError(t, db.LoadFromJSON([]byte(`{
		"airports": [
			{"code": "JFK", "name": "JFK", "city": "NYC", "country": "USA"},
			{"code": "LAX", "name": "LAX", "city": "LA", "country": "USA"}
		]
	}`)))

	all := db.All()
	assert.Len(t, all, 2)
}
