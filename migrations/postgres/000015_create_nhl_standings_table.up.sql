CREATE TABLE IF NOT EXISTS nhl_standings (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES nhl_teams(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    games_played SMALLINT NOT NULL DEFAULT 0,
    wins SMALLINT NOT NULL DEFAULT 0,
    losses SMALLINT NOT NULL DEFAULT 0,
    ot_losses SMALLINT NOT NULL DEFAULT 0,
    points SMALLINT NOT NULL DEFAULT 0,
    point_pct DOUBLE PRECISION,
    division_name VARCHAR(100) NOT NULL DEFAULT '',
    conference_name VARCHAR(100) NOT NULL DEFAULT '',
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_nhl_standings_team_date UNIQUE (team_id, date)
);

CREATE INDEX IF NOT EXISTS idx_nhl_standings_date ON nhl_standings (date);
