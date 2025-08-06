ALTER TABLE clusters
    ADD COLUMN IF NOT EXISTS max_requests_per_connection integer;