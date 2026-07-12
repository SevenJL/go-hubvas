ALTER TABLE users
    ADD COLUMN IF NOT EXISTS security_version BIGINT NOT NULL DEFAULT 1;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS chk_users_security_version;
ALTER TABLE users
    ADD CONSTRAINT chk_users_security_version CHECK (security_version > 0);
