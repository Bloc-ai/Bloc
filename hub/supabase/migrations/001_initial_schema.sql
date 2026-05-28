-- Migration: 001_initial_schema.sql
-- Codifying the existing Supabase schema for version control.
-- Note: Since the tables already exist in your live database, running this directly on your live DB will cause "relation already exists" errors.
-- These files are meant for version controlling your schema locally in git or setting up a fresh environment.

CREATE TABLE IF NOT EXISTS public.profiles (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  auth_id      UUID UNIQUE NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  username     TEXT UNIQUE NOT NULL,
  display_name TEXT,
  bio          TEXT,
  location     TEXT,
  twitter      TEXT,
  linkedin     TEXT,
  avatar_url   TEXT,
  role         TEXT DEFAULT 'community',
  created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS public.recipes (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  auth_id          UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  creator          TEXT NOT NULL,
  name             TEXT NOT NULL,
  description      TEXT,
  base_model       TEXT,
  min_vram         TEXT,
  target_platform  TEXT,
  yaml_content     TEXT,
  tested_commit    TEXT,
  compat_builds    JSONB DEFAULT '[]',
  created_at       TIMESTAMPTZ DEFAULT now(),
  CONSTRAINT unique_user_recipe UNIQUE (creator, name)
);

CREATE TABLE IF NOT EXISTS public.stars (
  user_id   UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  recipe_id UUID NOT NULL REFERENCES public.recipes(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, recipe_id)
);

CREATE TABLE IF NOT EXISTS public.follows (
  follower_id  UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  following_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  PRIMARY KEY (follower_id, following_id)
);
