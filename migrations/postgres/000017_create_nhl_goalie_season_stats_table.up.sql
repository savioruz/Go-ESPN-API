CREATE TABLE IF NOT EXISTS nhl_goalie_season_stats (
    id BIGSERIAL PRIMARY KEY,
    player_id BIGINT NOT NULL REFERENCES nhl_players(id) ON DELETE CASCADE,
    season VARCHAR(20) NOT NULL,
    games_played SMALLINT NOT NULL DEFAULT 0,
    wins SMALLINT NOT NULL DEFAULT 0,
    losses SMALLINT NOT NULL DEFAULT 0,
    save_pct DOUBLE PRECISION,
    gaa DOUBLE PRECISION,
    shutouts SMALLINT NOT NULL DEFAULT 0,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_nhl_goalie_season_stats UNIQUE (player_id, season)
);

CREATE INDEX IF NOT EXISTS idx_nhl_goalie_season_stats_season ON nhl_goalie_season_stats (season);
