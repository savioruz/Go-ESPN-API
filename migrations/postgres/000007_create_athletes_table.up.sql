CREATE TABLE IF NOT EXISTS athletes (
    id BIGSERIAL PRIMARY KEY,
    espn_id VARCHAR(50) NOT NULL,
    uid VARCHAR(100) NOT NULL DEFAULT '',
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    short_name VARCHAR(50) NOT NULL DEFAULT '',
    team_id BIGINT REFERENCES teams(id) ON DELETE SET NULL,
    position VARCHAR(50) NOT NULL DEFAULT '',
    position_abbreviation VARCHAR(10) NOT NULL DEFAULT '',
    jersey VARCHAR(10) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    height VARCHAR(20) NOT NULL DEFAULT '',
    weight INTEGER,
    age SMALLINT,
    birth_date DATE,
    birth_place VARCHAR(100) NOT NULL DEFAULT '',
    headshot VARCHAR(500) NOT NULL DEFAULT '',
    links JSONB NOT NULL DEFAULT '[]'::jsonb,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_athletes_espn_id UNIQUE (espn_id)
);

CREATE INDEX IF NOT EXISTS idx_athletes_team_id ON athletes (team_id);
