-- 001_create_users.sql
-- Creates the users table with appropriate constraints and indexes.

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL    PRIMARY KEY,
    username      VARCHAR(50)  NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    avatar_url    TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT uq_users_username UNIQUE (username),
    CONSTRAINT uq_users_email    UNIQUE (email)
);

-- Case-insensitive lookup by username (common login pattern).
CREATE INDEX IF NOT EXISTS idx_users_username_lower ON users (LOWER(username));

-- Lookup by email for login.
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));
