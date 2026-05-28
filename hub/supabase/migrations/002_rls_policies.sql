-- Migration: 002_rls_policies.sql
-- Enables RLS and creates policies.
-- Safe to run on your live database if RLS is not yet set up or you want to update it.

-- 1. Enable RLS on all tables (safe to run if already enabled)
ALTER TABLE public.profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.recipes ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.stars ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.follows ENABLE ROW LEVEL SECURITY;

-- 2. Drop existing policies to prevent conflicts on execution
DROP POLICY IF EXISTS "profiles_read_all" ON public.profiles;
DROP POLICY IF EXISTS "profiles_update_own" ON public.profiles;
DROP POLICY IF EXISTS "recipes_read_all" ON public.recipes;
DROP POLICY IF EXISTS "recipes_insert_own" ON public.recipes;
DROP POLICY IF EXISTS "recipes_update_own" ON public.recipes;
DROP POLICY IF EXISTS "recipes_delete_own" ON public.recipes;
DROP POLICY IF EXISTS "stars_read_all" ON public.stars;
DROP POLICY IF EXISTS "stars_insert_own" ON public.stars;
DROP POLICY IF EXISTS "stars_delete_own" ON public.stars;
DROP POLICY IF EXISTS "follows_read_all" ON public.follows;
DROP POLICY IF EXISTS "follows_insert_own" ON public.follows;
DROP POLICY IF EXISTS "follows_delete_own" ON public.follows;

-- 3. Create fresh policies

-- Profiles policies
CREATE POLICY "profiles_read_all" ON public.profiles FOR SELECT USING (true);
CREATE POLICY "profiles_update_own" ON public.profiles FOR UPDATE USING (auth.uid() = auth_id);

-- Recipes policies
CREATE POLICY "recipes_read_all" ON public.recipes FOR SELECT USING (true);
CREATE POLICY "recipes_insert_own" ON public.recipes FOR INSERT WITH CHECK (auth.uid() = auth_id);
CREATE POLICY "recipes_update_own" ON public.recipes FOR UPDATE USING (auth.uid() = auth_id);
CREATE POLICY "recipes_delete_own" ON public.recipes FOR DELETE USING (auth.uid() = auth_id);

-- Stars policies
CREATE POLICY "stars_read_all" ON public.stars FOR SELECT USING (true);
CREATE POLICY "stars_insert_own" ON public.stars FOR INSERT WITH CHECK (auth.uid() = user_id);
CREATE POLICY "stars_delete_own" ON public.stars FOR DELETE USING (auth.uid() = user_id);

-- Follows policies
CREATE POLICY "follows_read_all" ON public.follows FOR SELECT USING (true);
CREATE POLICY "follows_insert_own" ON public.follows FOR INSERT WITH CHECK (auth.uid() = follower_id);
CREATE POLICY "follows_delete_own" ON public.follows FOR DELETE USING (auth.uid() = follower_id);
