import { NextRequest, NextResponse } from "next/server";
import { getSupabaseAnon } from "@/lib/supabase-server";

// F-11: CORS restricted to the Bloc Hub origin.
const CORS_ORIGIN = "https://bloc-hub.com";

// F-10: Allowlist for reflected path values — only alphanumeric, dash, dot, underscore.
// Prevents reflected injection via crafted author/recipe path segments.
const SAFE_SEGMENT_RE = /^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,99}$/;

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ author: string; recipe: string }> }
) {
  const { author, recipe } = await params;

  // F-10: Validate path segments before reflecting them in any error message
  if (!author || !recipe || !SAFE_SEGMENT_RE.test(author) || !SAFE_SEGMENT_RE.test(recipe)) {
    return NextResponse.json(
      { error: "Invalid recipe identifier" },
      { status: 400 }
    );
  }

  // P-03: Use module-scoped singleton — not a new client per request
  const supabase = getSupabaseAnon();
  if (!supabase) {
    return NextResponse.json(
      { error: "Service unavailable — database not configured" },
      { status: 503 }
    );
  }

  const { data, error } = await supabase
    .from("recipes")
    .select("name, creator, description, yaml_content, min_vram, target_platform, base_model, compat_builds")
    .eq("creator", author)
    .eq("name", recipe)
    .maybeSingle();

  if (error) {
    console.error("[API] /api/recipes/[author]/[recipe] Supabase error:", error);
    return NextResponse.json(
      { error: "Database error" },
      { status: 500 }
    );
  }

  if (!data) {
    // F-10: Don't echo raw user input — use a generic not-found message
    return NextResponse.json(
      { error: "Recipe not found" },
      { status: 404 }
    );
  }

  // Return the envelope format the CLI expects:
  // { yaml_content: "...", name: "...", creator: "..." }
  return NextResponse.json(
    {
      yaml_content: data.yaml_content,
      name: data.name,
      creator: data.creator,
      description: data.description,
      min_vram: data.min_vram,
      target_platform: data.target_platform,
      base_model: data.base_model,
      compat_builds: data.compat_builds ?? [],
    },
    {
      status: 200,
      headers: {
        // F-11: Restrict CORS to Hub domain — CLI is a Go HTTP client that doesn't use CORS
        "Access-Control-Allow-Origin": CORS_ORIGIN,
        "Access-Control-Allow-Methods": "GET",
        // P-18: Recipe YAML is immutable once a PR is merged — use a long CDN TTL.
        // If a recipe must be updated, the slug changes (new PR), so cache is naturally busted.
        "Cache-Control": "public, s-maxage=86400, stale-while-revalidate=604800",
        "Vary": "Origin",
      },
    }
  );
}

// Handle CORS preflight
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
