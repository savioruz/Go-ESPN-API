CREATE TABLE IF NOT EXISTS transactions (
    id BIGSERIAL PRIMARY KEY,
    league_id BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id BIGINT REFERENCES teams(id) ON DELETE SET NULL,
    espn_id VARCHAR(100) NOT NULL DEFAULT '',
    date DATE,
    description TEXT NOT NULL,
    type VARCHAR(100) NOT NULL DEFAULT '',
    athlete_name VARCHAR(100) NOT NULL DEFAULT '',
    athlete_espn_id VARCHAR(50) NOT NULL DEFAULT '',
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_league_id ON transactions (league_id);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions (date);
CREATE INDEX IF NOT EXISTS idx_transactions_athlete_espn_id ON transactions (athlete_espn_id);
