CREATE TABLE IF NOT EXISTS teams (
    id BIGSERIAL PRIMARY KEY,
    league_id BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    espn_id VARCHAR(50) NOT NULL,
    uid VARCHAR(100) NOT NULL DEFAULT '',
    slug VARCHAR(100) NOT NULL DEFAULT '',
    abbreviation VARCHAR(10) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    short_display_name VARCHAR(50) NOT NULL DEFAULT '',
    name VARCHAR(50) NOT NULL DEFAULT '',
    nickname VARCHAR(50) NOT NULL DEFAULT '',
    location VARCHAR(100) NOT NULL DEFAULT '',
    color VARCHAR(10) NOT NULL DEFAULT '',
    alternate_color VARCHAR(10) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_all_star BOOLEAN NOT NULL DEFAULT FALSE,
    logos JSONB NOT NULL DEFAULT '[]'::jsonb,
    links JSONB NOT NULL DEFAULT '[]'::jsonb,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_teams_league_espn UNIQUE (league_id, espn_id)
);

CREATE INDEX IF NOT EXISTS idx_teams_league_id ON teams (league_id);
CREATE INDEX IF NOT EXISTS idx_teams_espn_id ON teams (espn_id);
