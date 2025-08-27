-- Drop retry fields from jobs table
ALTER TABLE jobs 
DROP COLUMN IF EXISTS max_retries,
DROP COLUMN IF EXISTS retry_count,
DROP COLUMN IF EXISTS retry_after;