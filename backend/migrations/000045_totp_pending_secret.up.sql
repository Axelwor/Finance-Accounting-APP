-- B-01 fix: totp_pending_secret holds a newly-generated secret that has not
-- yet been verified by the user. Setup2FA writes here; Setup2FAVerify copies
-- it to totp_secret and clears the pending value. This two-phase swap ensures
-- the old secret stays valid until the new one is proven, preventing
-- irrecoverable lockout if the setup response is lost.

ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_pending_secret TEXT;
