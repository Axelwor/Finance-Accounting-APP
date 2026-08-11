-- m-006: Two-factor authentication (TOTP, RFC 6238).
-- totp_secret holds the base32-encoded shared secret (NULL when 2FA off).
-- totp_enabled flips on only after the user verifies a code during setup.

ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT false;
