CREATE TABLE IF NOT EXISTS news_articles (
    id BIGSERIAL PRIMARY KEY,
    espn_id VARCHAR(100) NOT NULL,
    headline VARCHAR(500) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    story TEXT NOT NULL DEFAULT '',
    published TIMESTAMPTZ,
    last_modified TIMESTAMPTZ,
    type VARCHAR(50) NOT NULL DEFAULT '',
    league_id BIGINT REFERENCES leagues(id) ON DELETE SET NULL,
    categories JSONB NOT NULL DEFAULT '[]'::jsonb,
    images JSONB NOT NULL DEFAULT '[]'::jsonb,
    links JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_news_articles_espn_id UNIQUE (espn_id)
);

CREATE INDEX IF NOT EXISTS idx_news_articles_published ON news_articles (published);
CREATE INDEX IF NOT EXISTS idx_news_articles_league_id ON news_articles (league_id);
