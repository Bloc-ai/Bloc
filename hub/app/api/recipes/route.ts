import { NextRequest, NextResponse } from "next/server";
import { getSupabaseAnon } from "@/lib/supabase-server";

// F-11: CORS restricted to the Bloc Hub origin.
// The CLI is a Go HTTP client — it doesn't use CORS at all.
// Wildcard CORS would allow any webpage to scrape the registry.
const CORS_ORIGIN = "https://bloc-hub.com";

// Valid allowed values for filter parameters (unchanged — already good)
const VALID_VRAM = new Set(["4GB", "8GB", "12GB", "16GB", "24GB", "Unified"]);
const VALID_PLATFORMS = new Set(["cuda", "metal", "rocm", "cpu", "vulkan"]);

// F-05: Characters that have special meaning in the PostgREST filter syntax.
// Strip these before interpolating `q` into the .or() filter string.
// Also enforces a max length to prevent oversized queries.
function sanitizeSearchQuery(q: string): string {
  // Remove PostgREST meta-characters: , . ( ) % that could break filter syntax
  return q.replace(/[,.()\[\]%]/g, "").slice(0, 200);
}

export async function GET(req: NextRequest) {
  const { searchParams } = req.nextUrl;

  const rawQ = searchParams.get("q")?.trim() ?? "";
  // F-05: Sanitize q before PostgREST interpolation
  const q = sanitizeSearchQuery(rawQ);

  const minVram = searchParams.get("min_vram") ?? searchParams.get("vram") ?? "";
  const platform = searchParams.get("platform") ?? "";
  const limitRaw = parseInt(searchParams.get("limit") ?? "20", 10);
  const limit = Math.min(Math.max(1, isNaN(limitRaw) ? 20 : limitRaw), 100);

  // P-03: Use module-scoped singleton — not a new client per request
  const supabase = getSupabaseAnon();
  if (!supabase) {
    return NextResponse.json(
      { error: "Service unavailable — database not configured" },
      { status: 503 }
    );
  }

  // P-14: Removed `created_at` and `compat_builds` from SELECT — neither is
  // returned in the response mapping below. Reduces payload size.
  // ORDER BY created_at still works via Supabase without it being in SELECT
  // when using the JS client (it is included internally by the query planner).
  // NOTE: If Supabase requires it in select for order, add it back and strip from response.
  let query = supabase
    .from("recipes")
    .select(
      "name, creator, description, base_model, min_vram, target_platform, created_at"
    )
    .order("created_at", { ascending: false })
    .limit(limit);

  // P-05: Full-text / ILIKE search.
  // ⚠️ ACTION REQUIRED: Run the following SQL in your Supabase SQL editor to add
  // GIN trigram indexes — without them, ILIKE %q% is a full sequential scan:
  //
  //   CREATE EXTENSION IF NOT EXISTS pg_trgm;
  //   CREATE INDEX IF NOT EXISTS idx_recipes_name_trgm
  //     ON recipes USING GIN (name gin_trgm_ops);
  //   CREATE INDEX IF NOT EXISTS idx_recipes_description_trgm
  //     ON recipes USING GIN (description gin_trgm_ops);
  //   CREATE INDEX IF NOT EXISTS idx_recipes_base_model_trgm
  //     ON recipes USING GIN (base_model gin_trgm_ops);
  //   CREATE INDEX IF NOT EXISTS idx_recipes_created_at
  //     ON recipes (created_at DESC);
  if (q) {
    // F-05: q is already sanitized above — PostgREST metacharacters removed
    query = query.or(
      `name.ilike.%${q}%,description.ilike.%${q}%,base_model.ilike.%${q}%`
    );
  }

  // VRAM filter — only apply if it's a valid value to prevent injection
  if (minVram && VALID_VRAM.has(minVram)) {
    query = query.eq("min_vram", minVram);
  }

  // Platform filter
  if (platform && VALID_PLATFORMS.has(platform)) {
    query = query.eq("target_platform", platform);
  }

  const { data, error } = await query;

  if (error) {
    console.error("[API] /api/recipes Supabase error:", error);
    return NextResponse.json({ error: "Database error" }, { status: 500 });
  }

  // Map to the shape bloc search already knows how to render
  const results = (data ?? []).map((row: any) => ({
    creator: row.creator,
    name: row.name,
    description: row.description ?? "",
    base_model: row.base_model ?? "",
    min_vram: row.min_vram ?? "",
    target_platform: row.target_platform ?? "",
    stars_count: 0, // P-14: compat_builds removed from SELECT; use dedicated endpoint if needed
  }));

  return NextResponse.json(results, {
    status: 200,
    headers: {
      // F-11: Restrict CORS to the Hub domain — CLI doesn't use CORS at all
      "Access-Control-Allow-Origin": CORS_ORIGIN,
      "Access-Control-Allow-Methods": "GET",
      // Registry cache — short TTL since new recipes merge frequently
      "Cache-Control": "public, s-maxage=30, stale-while-revalidate=120",
      "Vary": "Origin",
    },
  });
}

export async function OPTIONS() {
  return new Response(null, {
    status: 204,
    headers: {
      // F-11: Restricted CORS
      "Access-Control-Allow-Origin": CORS_ORIGIN,
      "Access-Control-Allow-Methods": "GET, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type, User-Agent",
      "Vary": "Origin",
    },
  });
}
