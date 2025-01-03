-- URL types enum
CREATE TYPE url_type AS ENUM (
    'original_name',
    'default',
    'random',
    'date',
    'uuid',
    'gfycat'
);

-- URL mappings table for different URL types
CREATE TABLE file_urls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES uploaded_files(id) ON DELETE CASCADE,
    url_type url_type NOT NULL,
    url_value TEXT NOT NULL,  -- The actual URL path/identifier
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure unique URL values per type
    CONSTRAINT unique_url_per_type UNIQUE (url_type, url_value)
);

-- Indexes for performance
CREATE INDEX idx_file_urls_file_id ON file_urls(file_id);
CREATE INDEX idx_file_urls_url_value ON file_urls(url_value);
