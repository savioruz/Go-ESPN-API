CREATE TABLE IF NOT EXISTS events (
    id BIGSERIAL PRIMARY KEY,
    league_id BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    venue_id BIGINT REFERENCES venues(id) ON DELETE SET NULL,
    espn_id VARCHAR(50) NOT NULL,
    uid VARCHAR(100) NOT NULL DEFAULT '',
    date TIMESTAMPTZ NOT NULL,
    name VARCHAR(200) NOT NULL,
    short_name VARCHAR(100) NOT NULL DEFAULT '',
    season_year INTEGER NOT NULL,
    season_type SMALLINT NOT NULL DEFAULT 2,
    season_slug VARCHAR(50) NOT NULL DEFAULT '',
    week SMALLINT,
    status VARCHAR(20) NOT NULL DEFAULT 'scheduled',
    status_detail VARCHAR(100) NOT NULL DEFAULT '',
    clock VARCHAR(20) NOT NULL DEFAULT '',
    period SMALLINT,
    attendance INTEGER,
    broadcasts JSONB NOT NULL DEFAULT '[]'::jsonb,
    links JSONB NOT NULL DEFAULT '[]'::jsonb,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_events_league_espn UNIQUE (league_id, espn_id)
);

CREATE INDEX IF NOT EXISTS idx_events_league_date_status ON events (league_id, date, status);
CREATE INDEX IF NOT EXISTS idx_events_date ON events (date);
CREATE INDEX IF NOT EXISTS idx_events_status ON events (status);
CREATE INDEX IF NOT EXISTS idx_events_espn_id ON events (espn_id);
CREATE INDEX IF NOT EXISTS idx_events_season_year ON events (season_year);
