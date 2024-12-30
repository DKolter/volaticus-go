CREATE TABLE shortened_urls (
                                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                original_url TEXT NOT NULL,
                                short_code VARCHAR(10) NOT NULL UNIQUE,
                                created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                                last_accessed_at TIMESTAMP WITH TIME ZONE,
                                access_count INTEGER DEFAULT 0
);

-- Index for quick lookups by short_code
CREATE INDEX idx_short_urls_code ON shortened_urls(short_code);