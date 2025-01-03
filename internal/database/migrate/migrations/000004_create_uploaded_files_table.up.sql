-- Main uploaded files table
CREATE TABLE uploaded_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- File information
    original_name TEXT NOT NULL,
    unix_filename BIGINT NOT NULL UNIQUE,  -- Using BIGINT for unix timestamp filename
    mime_type VARCHAR(255),
    file_size BIGINT,
    
    -- Metadata
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,  -- Optional foreign key to users
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_accessed_at TIMESTAMP WITH TIME ZONE,
    access_count INTEGER DEFAULT 0,
    expires_at TIMESTAMP WITH TIME ZONE,  -- New expiration field
    
    -- Add constraint to ensure unix_filename is unique
    CONSTRAINT unique_unix_filename UNIQUE (unix_filename)
);

-- Indexes for performance
CREATE INDEX idx_uploaded_files_unix_filename ON uploaded_files(unix_filename);
CREATE INDEX idx_uploaded_files_user_id ON uploaded_files(user_id);
CREATE INDEX idx_uploaded_files_created_at ON uploaded_files(created_at);
CREATE INDEX idx_uploaded_files_expires_at ON uploaded_files(expires_at);
