package model

import (
	"encoding/json"

	"go-espn-api/shared/model"
)

const (
	TableName  = "nhl_teams"
	EntityName = "nhl_team"

	FieldID           = "id"
	FieldTeamID       = "team_id"
	FieldAbbreviation = "abbreviation"
	FieldName         = "name"
	FieldFullName     = "full_name"
	FieldFranchiseID  = "franchise_id"
	FieldIsActive     = "is_active"
	FieldRawData      = "raw_data"
)

type NHLTeam struct {
	ID           int64           `db:"id"`
	TeamID       string          `db:"team_id"`
	Abbreviation string          `db:"abbreviation"`
	Name         string          `db:"name"`
	FullName     string          `db:"full_name"`
	FranchiseID  string          `db:"franchise_id"`
	IsActive     bool            `db:"is_active"`
	RawData      json.RawMessage `db:"raw_data"`
	model.Timestamp
}
