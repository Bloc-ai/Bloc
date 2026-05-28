-- Migration: 003_telemetry_schema.sql
-- Create telemetry logs table to store individual anonymous runs from CLI.
-- Safe to apply to your live database.

-- 1. Create Telemetry Table
CREATE TABLE IF NOT EXISTS public.telemetry (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recipe_id       UUID NOT NULL REFERENCES public.recipes(id) ON DELETE CASCADE,
  success         BOOLEAN NOT NULL,
  tokens_per_sec  NUMERIC,
  peak_vram_mb    NUMERIC,
  os              TEXT,
  arch            TEXT,
  llama_build     TEXT,
  created_at      TIMESTAMPTZ DEFAULT now()
);

-- 2. Enable Row-Level Security (safe to run if already enabled)
ALTER TABLE public.telemetry ENABLE ROW LEVEL SECURITY;

-- 3. Drop existing policies to prevent conflicts on execution
DROP POLICY IF EXISTS "telemetry_read_all" ON public.telemetry;
DROP POLICY IF EXISTS "telemetry_insert_service" ON public.telemetry;

-- 4. Create fresh RLS policies
-- Anyone can read benchmarks, but direct inserts are blocked for public/anon users (handled securely via Edge Function)
CREATE POLICY "telemetry_read_all" ON public.telemetry FOR SELECT USING (true);
CREATE POLICY "telemetry_insert_service" ON public.telemetry FOR INSERT WITH CHECK (false);
