package model

import (
	"encoding/json"

	"go-espn-api/shared/model"
)

const (
	TableName  = "venues"
	EntityName = "venue"

	FieldID       = "id"
	FieldESPNID   = "espn_id"
	FieldName     = "name"
	FieldCity     = "city"
	FieldState    = "state"
	FieldCountry  = "country"
	FieldIsIndoor = "is_indoor"
	FieldCapacity = "capacity"
	FieldRawData  = "raw_data"
)

type Venue struct {
	ID       int64           `db:"id"`
	ESPNID   string          `db:"espn_id"`
	Name     string          `db:"name"`
	City     string          `db:"city"`
	State    string          `db:"state"`
	Country  string          `db:"country"`
	IsIndoor bool            `db:"is_indoor"`
	Capacity *int            `db:"capacity"`
	RawData  json.RawMessage `db:"raw_data"`
	model.Timestamp
}
