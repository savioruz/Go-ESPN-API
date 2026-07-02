CREATE TABLE IF NOT EXISTS injuries (
    id BIGSERIAL PRIMARY KEY,
    league_id BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id BIGINT REFERENCES teams(id) ON DELETE SET NULL,
    espn_id VARCHAR(100) NOT NULL DEFAULT '',
    athlete_espn_id VARCHAR(50) NOT NULL DEFAULT '',
    athlete_name VARCHAR(100) NOT NULL,
    position VARCHAR(50) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'other',
    status_display VARCHAR(100) NOT NULL DEFAULT '',
    description VARCHAR(500) NOT NULL DEFAULT '',
    injury_type VARCHAR(100) NOT NULL DEFAULT '',
    injury_date DATE,
    return_date DATE,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_injuries_league_id ON injuries (league_id);
CREATE INDEX IF NOT EXISTS idx_injuries_team_id ON injuries (team_id);
CREATE INDEX IF NOT EXISTS idx_injuries_athlete_espn_id ON injuries (athlete_espn_id);
CREATE INDEX IF NOT EXISTS idx_injuries_status ON injuries (status);
