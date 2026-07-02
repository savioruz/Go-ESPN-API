CREATE TABLE IF NOT EXISTS nhl_teams (
    id BIGSERIAL PRIMARY KEY,
    team_id VARCHAR(50) NOT NULL,
    abbreviation VARCHAR(10) NOT NULL,
    name VARCHAR(100) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    franchise_id VARCHAR(50) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_nhl_teams_team_id UNIQUE (team_id)
);
