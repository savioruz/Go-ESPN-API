CREATE TABLE IF NOT EXISTS competitors (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    home_away VARCHAR(4) NOT NULL,
    score VARCHAR(10) NOT NULL DEFAULT '',
    winner BOOLEAN,
    line_scores JSONB NOT NULL DEFAULT '[]'::jsonb,
    records JSONB NOT NULL DEFAULT '[]'::jsonb,
    statistics JSONB NOT NULL DEFAULT '[]'::jsonb,
    leaders JSONB NOT NULL DEFAULT '[]'::jsonb,
    "order" SMALLINT NOT NULL DEFAULT 0,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_competitors_event_team UNIQUE (event_id, team_id)
);

CREATE INDEX IF NOT EXISTS idx_competitors_event_id ON competitors (event_id);
CREATE INDEX IF NOT EXISTS idx_competitors_team_id ON competitors (team_id);
