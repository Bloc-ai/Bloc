import { createClient, SupabaseClient } from "@supabase/supabase-js";

// P-03: Module-scoped Supabase singleton — created once per warm serverless container.
// Eliminates the cost of calling createClient() on every request.
let _anonClient: SupabaseClient | null = null;
let _adminClient: SupabaseClient | null = null;

/**
 * Returns a Supabase client using the public anon key (for reads).
 * Safe for public recipe endpoints — RLS ensures only public rows are returned.
 */
export function getSupabaseAnon(): SupabaseClient | null {
  if (_anonClient) return _anonClient;
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
  if (!url || !key) return null;
  _anonClient = createClient(url, key, { auth: { persistSession: false } });
  return _anonClient;
}

/**
 * Returns a Supabase client using the service role key (for writes that bypass RLS).
 * F-04: Fails hard if SUPABASE_SERVICE_KEY is absent — never silently falls back
 * to the anon key (which would be a silent security degradation).
 *
 * Note: SUPABASE_SERVICE_KEY must NOT have the NEXT_PUBLIC_ prefix — it is a
 * server-only secret and must never be bundled into client-side JS.
 */
export function getSupabaseAdmin(): SupabaseClient | null {
  if (_adminClient) return _adminClient;
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.SUPABASE_SERVICE_KEY; // F-04: no anon fallback
  if (!url || !key) {
    // Log to server-side only — never expose to clients
    console.error(
      "[bloc] SUPABASE_SERVICE_KEY is not set. Telemetry writes will be skipped."
    );
    return null;
  }
  _adminClient = createClient(url, key, { auth: { persistSession: false } });
  return _adminClient;
}
