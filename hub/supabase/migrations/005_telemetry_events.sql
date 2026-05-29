-- Migration: 005_telemetry_events.sql
-- Replaces old mismatched telemetry table with telemetry_events matching CLI/API schema.
-- Safe to apply to your live database.

DROP TABLE IF EXISTS public.telemetry;

CREATE TABLE IF NOT EXISTS public.telemetry_events (
  id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recipe_id                 UUID NOT NULL REFERENCES public.recipes(id) ON DELETE CASCADE,
  success                   BOOLEAN NOT NULL,
  tokens_per_sec_generation  NUMERIC,
  tokens_per_sec_prefill    NUMERIC,
  peak_vram_mb              NUMERIC,
  duration_seconds          NUMERIC,
  os                        TEXT,
  arch                      TEXT,
  cli_version               TEXT,
  recorded_at               TIMESTAMPTZ DEFAULT now()
);

-- Enable RLS
ALTER TABLE public.telemetry_events ENABLE ROW LEVEL SECURITY;

-- Allow public read access to benchmarks
DROP POLICY IF EXISTS "read_all" ON public.telemetry_events;
CREATE POLICY "read_all" ON public.telemetry_events FOR SELECT USING (true);

-- Only allow insert via Service Role Key (bypasses RLS)
DROP POLICY IF EXISTS "insert_service" ON public.telemetry_events;
CREATE POLICY "insert_service" ON public.telemetry_events FOR INSERT WITH CHECK (false);

-- Index for fast per-recipe aggregation queries
CREATE INDEX IF NOT EXISTS idx_telemetry_recipe_id ON public.telemetry_events (recipe_id, recorded_at DESC);
