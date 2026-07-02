package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"go-espn-api/infras/espn"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	competitorModel "go-espn-api/internal/domains/competitor/model"
	competitorRepo "go-espn-api/internal/domains/competitor/repository"
	eventModel "go-espn-api/internal/domains/event/model"
	eventRepo "go-espn-api/internal/domains/event/repository"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	teamRepo "go-espn-api/internal/domains/team/repository"
	venueModel "go-espn-api/internal/domains/venue/model"
	venueRepo "go-espn-api/internal/domains/venue/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// defaultSeasonType is the ESPN season-type fallback (2 = regular season).
const defaultSeasonType = 2

// ScoreboardService ingests scoreboard/event data from ESPN.
type ScoreboardService struct {
	espn        espn.ESPN
	db          *postgres.Connection
	otel        otel.Otel
	sports      sportRepo.Sport
	leagues     leagueRepo.League
	venues      venueRepo.Venue
	teams       teamRepo.Team
	events      eventRepo.Event
	competitors competitorRepo.Competitor
}

// NewScoreboardService constructs a ScoreboardService.
func NewScoreboardService(
	espnClient espn.ESPN,
	db *postgres.Connection,
	otl otel.Otel,
	sports sportRepo.Sport,
	leagues leagueRepo.League,
	venues venueRepo.Venue,
	teams teamRepo.Team,
	events eventRepo.Event,
	competitors competitorRepo.Competitor,
) *ScoreboardService {
	return &ScoreboardService{
		espn:        espnClient,
		db:          db,
		otel:        otl,
		sports:      sports,
		leagues:     leagues,
		venues:      venues,
		teams:       teams,
		events:      events,
		competitors: competitors,
	}
}

// IngestScoreboard fetches and upserts scoreboard data for a sport, league, and
// optional date (YYYYMMDD). The whole run is a single transaction.
func (s *ScoreboardService) IngestScoreboard(ctx context.Context, sport, league, date string) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.Scoreboard")
	defer scope.End()

	scope.SetAttributes(map[string]any{"sport": sport, "league": league, "date": date})

	result := newResult()

	err := withTx(s.db, func(tx *sqlx.Tx) error {
		leagueID, err := resolveSportLeague(ctx, tx, s.sports, s.leagues, sport, league)
		if err != nil {
			return err
		}

		resp, err := s.espn.GetScoreboard(ctx, sport, league, date, 0)
		if err != nil {
			return fmt.Errorf("fetch scoreboard: %w", err)
		}

		var root map[string]any
		if err := json.Unmarshal(resp.Data, &root); err != nil {
			return fmt.Errorf("decode scoreboard: %w", err)
		}

		events := mslice(root, "events")
		scope.SetAttribute("events_found", len(events))

		if len(events) == 0 {
			log.Info().Str("sport", sport).Str("league", league).Str("date", date).Msg("no_events_found")

			return nil
		}

		for _, raw := range events {
			eventData := asMap(raw)
			if err := s.ingestEvent(ctx, tx, leagueID, eventData, &result); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		scope.TraceError(err)

		return IngestionResult{}, fmt.Errorf("failed to ingest scoreboard: %w", err)
	}

	scope.SetAttributes(map[string]any{"created": result.Created, "updated": result.Updated, "errors": result.Errors})

	return result, nil
}

// ingestEvent upserts a single event plus its venue and competitors. A parse
// error (e.g. missing espn id) increments result.Errors and returns nil so the
// run continues; a DB error is returned and aborts the transaction.
func (s *ScoreboardService) ingestEvent(ctx context.Context, tx *sqlx.Tx, leagueID int64, eventData map[string]any, result *IngestionResult) error {
	espnID := mid(eventData, "id")
	if espnID == "" {
		result.Errors++

		return nil
	}

	competitions := mslice(eventData, "competitions")

	competition := map[string]any{}
	if len(competitions) > 0 {
		competition = asMap(competitions[0])
	}

	statusData := mmap(eventData, "status")
	status, statusDetail := parseEventStatus(statusData)
	season := mmap(eventData, "season")
	date := parseEventTime(mstr(eventData, "date"))

	venueID, err := s.upsertVenue(ctx, tx, mmap(competition, "venue"))
	if err != nil {
		return err
	}

	ev := eventModel.Event{
		LeagueID:     leagueID,
		VenueID:      venueID,
		ESPNID:       espnID,
		UID:          mstr(eventData, "uid"),
		Date:         date,
		Name:         mstr(eventData, "name"),
		ShortName:    mstr(eventData, "shortName"),
		SeasonYear:   mint(season, "year", date.Year()),
		SeasonType:   mint(season, "type", defaultSeasonType),
		SeasonSlug:   mstr(season, "slug"),
		Week:         mintPtr(mmap(eventData, "week"), "number"),
		Status:       status,
		StatusDetail: statusDetail,
		Clock:        mstr(statusData, "displayClock"),
		Period:       mintPtr(statusData, "period"),
		Attendance:   mintPtr(competition, "attendance"),
		Broadcasts:   rawObj(competition["broadcasts"], "[]"),
		Links:        rawObj(eventData["links"], "[]"),
		RawData:      rawObj(eventData, "{}"),
	}

	res, err := s.events.UpsertTx(ctx, tx, ev)
	if err != nil {
		return err
	}

	if err := s.competitors.DeleteByEventTx(ctx, tx, res.ID); err != nil {
		return err
	}

	competitors := mslice(competition, "competitors")
	for idx, craw := range competitors {
		if err := s.createCompetitor(ctx, tx, leagueID, res.ID, idx, asMap(craw)); err != nil {
			return err
		}
	}

	if res.Inserted {
		result.Created++
	} else {
		result.Updated++
	}

	return nil
}

// upsertVenue mirrors _parse_venue_data + _get_or_create_venue, returning the
// (nullable) venue id.
func (s *ScoreboardService) upsertVenue(ctx context.Context, tx *sqlx.Tx, venueData map[string]any) (*int64, error) {
	espnID := mid(venueData, "id")
	if espnID == "" {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	address := mmap(venueData, "address")

	country := "USA"
	if v, ok := address["country"].(string); ok {
		country = v
	}

	v := venueModel.Venue{
		ESPNID:   espnID,
		Name:     mstrOr(venueData, "fullName", "shortName"),
		City:     mstr(address, "city"),
		State:    mstr(address, "state"),
		Country:  country,
		IsIndoor: mbool(venueData, "indoor", true),
		Capacity: mintPtr(venueData, "capacity"),
		RawData:  rawObj(venueData, "{}"),
	}

	id, err := s.venues.UpsertTx(ctx, tx, v)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

// createCompetitor resolves (or minimally creates) the team then inserts the
// competitor row. Competitors with no team id are skipped (Python continue).
func (s *ScoreboardService) createCompetitor(ctx context.Context, tx *sqlx.Tx, leagueID, eventID int64, idx int, compData map[string]any) error {
	teamData := mmap(compData, "team")

	teamESPNID := mid(teamData, "id")
	if teamESPNID == "" {
		return nil
	}

	teamID, err := s.teams.GetOrCreateMinimalTx(ctx, tx, teamRepo.MinimalTeam{
		LeagueID:         leagueID,
		ESPNID:           teamESPNID,
		Abbreviation:     mstr(teamData, "abbreviation"),
		DisplayName:      mstrOr(teamData, "displayName", "name"),
		ShortDisplayName: mstr(teamData, "shortDisplayName"),
		Name:             mstr(teamData, "name"),
		Location:         mstr(teamData, "location"),
		Logos:            rawObj(teamData["logo"], "[]"),
	})
	if err != nil {
		return err
	}

	comp := competitorModel.Competitor{
		EventID:    eventID,
		TeamID:     teamID,
		HomeAway:   homeAwayFallback(mstr(compData, "homeAway"), idx),
		Score:      mscalarStr(compData, "score"),
		Winner:     mboolPtr(compData, "winner"),
		LineScores: rawObj(compData["linescores"], "[]"),
		Records:    rawObj(compData["records"], "[]"),
		Statistics: rawObj(compData["statistics"], "[]"),
		Leaders:    rawObj(compData["leaders"], "[]"),
		Order:      idx,
		RawData:    rawObj(compData, "{}"),
	}

	return s.competitors.InsertTx(ctx, tx, comp)
}
