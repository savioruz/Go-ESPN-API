CREATE TABLE IF NOT EXISTS leagues (
    id BIGSERIAL PRIMARY KEY,
    sport_id BIGINT NOT NULL REFERENCES sports(id) ON DELETE CASCADE,
    slug VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    abbreviation VARCHAR(20) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_leagues_sport_slug UNIQUE (sport_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_leagues_sport_id ON leagues (sport_id);
