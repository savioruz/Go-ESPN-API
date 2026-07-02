CREATE TABLE IF NOT EXISTS athlete_season_stats (
    id BIGSERIAL PRIMARY KEY,
    athlete_id BIGINT REFERENCES athletes(id) ON DELETE CASCADE,
    league_id BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    athlete_espn_id VARCHAR(50) NOT NULL,
    athlete_name VARCHAR(100) NOT NULL DEFAULT '',
    season_year INTEGER NOT NULL,
    season_type SMALLINT NOT NULL DEFAULT 2,
    stats JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_athlete_season_stats UNIQUE (league_id, athlete_espn_id, season_year, season_type)
);
