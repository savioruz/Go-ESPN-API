CREATE TABLE IF NOT EXISTS nhl_games (
    id BIGSERIAL PRIMARY KEY,
    game_id VARCHAR(50) NOT NULL,
    season VARCHAR(20) NOT NULL,
    game_type SMALLINT NOT NULL DEFAULT 2,
    date TIMESTAMPTZ NOT NULL,
    home_team_id BIGINT NOT NULL REFERENCES nhl_teams(id) ON DELETE CASCADE,
    away_team_id BIGINT NOT NULL REFERENCES nhl_teams(id) ON DELETE CASCADE,
    home_score SMALLINT,
    away_score SMALLINT,
    status VARCHAR(50) NOT NULL DEFAULT 'scheduled',
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_nhl_games_game_id UNIQUE (game_id)
);

CREATE INDEX IF NOT EXISTS idx_nhl_games_season ON nhl_games (season);
CREATE INDEX IF NOT EXISTS idx_nhl_games_date ON nhl_games (date);
CREATE INDEX IF NOT EXISTS idx_nhl_games_home_team_id ON nhl_games (home_team_id);
CREATE INDEX IF NOT EXISTS idx_nhl_games_away_team_id ON nhl_games (away_team_id);
