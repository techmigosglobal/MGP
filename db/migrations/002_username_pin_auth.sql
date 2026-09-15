-- Username + PIN authentication. Legacy email/password columns remain nullable
-- for a controlled migration window but are no longer used for authentication.
ALTER TABLE users ADD COLUMN IF NOT EXISTS username text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS pin_hash text;
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

UPDATE users
SET username = lower(split_part(email, '@', 1))
WHERE username IS NULL AND email IS NOT NULL;

UPDATE users
SET username = 'user_' || replace(left(id::text, 12), '-', '')
WHERE username IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS users_username_key ON users(lower(username));

CREATE TABLE IF NOT EXISTS auth_login_attempts (
  username text PRIMARY KEY,
  failed_attempts integer NOT NULL DEFAULT 0 CHECK (failed_attempts >= 0),
  locked_until timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);
