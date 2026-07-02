CREATE TABLE IF NOT EXISTS nhl_players (
    id BIGSERIAL PRIMARY KEY,
    player_id VARCHAR(50) NOT NULL,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    sweater_number VARCHAR(10) NOT NULL DEFAULT '',
    position VARCHAR(10) NOT NULL DEFAULT '',
    current_team_id BIGINT REFERENCES nhl_teams(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    headshot_url VARCHAR(500) NOT NULL DEFAULT '',
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_nhl_players_player_id UNIQUE (player_id)
);

CREATE INDEX IF NOT EXISTS idx_nhl_players_current_team_id ON nhl_players (current_team_id);
