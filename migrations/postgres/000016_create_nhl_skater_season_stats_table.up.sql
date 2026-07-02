CREATE TABLE IF NOT EXISTS nhl_skater_season_stats (
    id BIGSERIAL PRIMARY KEY,
    player_id BIGINT NOT NULL REFERENCES nhl_players(id) ON DELETE CASCADE,
    season VARCHAR(20) NOT NULL,
    games_played SMALLINT NOT NULL DEFAULT 0,
    goals SMALLINT NOT NULL DEFAULT 0,
    assists SMALLINT NOT NULL DEFAULT 0,
    points SMALLINT NOT NULL DEFAULT 0,
    plus_minus SMALLINT NOT NULL DEFAULT 0,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_nhl_skater_season_stats UNIQUE (player_id, season)
);

CREATE INDEX IF NOT EXISTS idx_nhl_skater_season_stats_season ON nhl_skater_season_stats (season);
