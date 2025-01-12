CREATE TABLE shortened_urls (
                                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                user_id UUID REFERENCES users(id) ON DELETE CASCADE,
                                original_url TEXT NOT NULL,
                                short_code VARCHAR(50) NOT NULL UNIQUE,
                                created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                                expires_at TIMESTAMP WITH TIME ZONE,
                                last_accessed_at TIMESTAMP WITH TIME ZONE,
                                access_count INTEGER DEFAULT 0,
                                is_vanity BOOLEAN DEFAULT FALSE,
                                is_active BOOLEAN DEFAULT TRUE
);

CREATE TABLE click_analytics (
                                 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                 url_id UUID REFERENCES shortened_urls(id) ON DELETE CASCADE,
                                 clicked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                                 referrer TEXT,
                                 user_agent TEXT,
                                 ip_address TEXT,
                                 country_code VARCHAR(2),
                                 city TEXT,
                                 region TEXT
);

-- Create indexes for performance
CREATE INDEX idx_shortened_urls_user ON shortened_urls(user_id);
CREATE INDEX idx_shortened_urls_code ON shortened_urls(short_code);
CREATE INDEX idx_shortened_urls_expires ON shortened_urls(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_click_analytics_url ON click_analytics(url_id);
CREATE INDEX idx_click_analytics_clicked ON click_analytics(clicked_at);
CREATE INDEX idx_click_analytics_country ON click_analytics(country_code);
CREATE INDEX idx_click_analytics_ip ON click_analytics(ip_address);