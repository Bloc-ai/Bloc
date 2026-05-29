-- Migration: 004_device_codes.sql
-- Stores pending CLI device authorization requests for the OAuth Device Flow.
-- Safe to apply to your live database.
--
-- HOW IT WORKS:
--   1. CLI POSTs to /api/auth/device → a row is inserted here with device_code + user_code
--   2. User visits bloc-hub.com/auth/device, enters user_code, signs in with GitHub
--   3. The Hub page POSTs to /api/auth/device/authorize → row is updated: authorized=true, access_token=<jwt>
--   4. CLI polls /api/auth/device/token with device_code → row found + authorized → returns token, row deleted
--
-- All access goes through the service role key (bypasses RLS).
-- Public/anon access is fully blocked via RLS policies.

CREATE TABLE IF NOT EXISTS public.device_codes (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  device_code   TEXT UNIQUE NOT NULL,       -- sent to CLI (UUID), used for polling
  user_code     TEXT NOT NULL,              -- shown to user (e.g. ABCD-1234), typed in browser
  authorized    BOOLEAN DEFAULT false,       -- set to true after user authorizes in browser
  access_token  TEXT,                        -- Supabase JWT stored after authorization
  username      TEXT,                        -- bloc username stored after authorization
  expires_at    TIMESTAMPTZ NOT NULL,        -- 15 minutes from creation
  created_at    TIMESTAMPTZ DEFAULT now()
);

-- Enable RLS — all access goes through the service role key only
ALTER TABLE public.device_codes ENABLE ROW LEVEL SECURITY;

-- Block all direct public access (service key bypasses this)
DROP POLICY IF EXISTS "no_public_access" ON public.device_codes;
CREATE POLICY "no_public_access" ON public.device_codes USING (false);

-- Index for fast user_code lookups (browser authorization)
CREATE INDEX IF NOT EXISTS idx_device_codes_user_code ON public.device_codes (user_code);
-- Index for polling by device_code
CREATE INDEX IF NOT EXISTS idx_device_codes_device_code ON public.device_codes (device_code);
