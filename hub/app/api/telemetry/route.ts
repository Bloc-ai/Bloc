import { NextRequest, NextResponse } from "next/server";
import { getSupabaseAdmin } from "@/lib/supabase-server";

// F-11: CORS restricted to the Bloc Hub origin.
// The CLI sends POST from Go's net/http — it does not use CORS.
// Wildcard CORS would allow any webpage to flood the telemetry endpoint
// with fabricated benchmark data.
const CORS_ORIGIN = "https://bloc-hub.com";

// Fields we accept — anything else is silently dropped
interface TelemetryPayload {
  event: string;
  cli_version: string;
  os: string;
  arch: string;
  recipe_id: string; // "author/recipe-name"
  success: boolean;
  tokens_per_sec_generation?: number;
  tokens_per_sec_prefill?: number;
  peak_vram_mb?: number;
  duration_seconds?: number;
  // F-12: session_id field removed — CLI no longer sends it.
  // The server never stores it even if a legacy CLI sends it.
}

// Valid OS and arch values to prevent injection/spoofing
const VALID_OS = new Set(["darwin", "linux", "windows", "freebsd"]);
const VALID_ARCH = new Set(["amd64", "arm64", "386", "arm"]);

// F-09: Safe segment pattern — same as CLI validation
const SAFE_SEGMENT_RE = /^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,99}$/;

export async function POST(req: NextRequest) {
  // F-18: Body size limit — Next.js default is 4MB; add explicit check
  const contentLength = req.headers.get("content-length");
  if (contentLength && parseInt(contentLength, 10) > 8192) {
    return NextResponse.json({ ok: true, stored: false }, { status: 200 });
  }

  // Parse body
  let body: Partial<TelemetryPayload>;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "Invalid JSON body" }, { status: 400 });
  }

  // Validate required fields
  if (!body.event || !body.recipe_id) {
    return NextResponse.json(
      { error: "Missing required fields: event, recipe_id" },
      { status: 400 }
    );
  }

  // Parse author/recipe from recipe_id
  const parts = (body.recipe_id as string).split("/");
  if (parts.length !== 2 || !parts[0] || !parts[1]) {
    return NextResponse.json(
      { error: "recipe_id must be in format author/recipe-name" },
      { status: 400 }
    );
  }
  const [author, recipeName] = parts;

  // F-09: Validate segments before using in DB query
  if (!SAFE_SEGMENT_RE.test(author) || !SAFE_SEGMENT_RE.test(recipeName)) {
    // Silently accept — don't error the CLI for malformed telemetry
    return NextResponse.json({ ok: true, stored: false }, { status: 200 });
  }

  // Build the benchmark entry — only store validated, allowlisted fields.
  // F-12: session_id is NOT stored even if sent.
  // Never store: IP, hostname, file paths, session_id, prompt content.
  const benchmarkEntry = {
    cli_version: String(body.cli_version ?? "unknown").slice(0, 50),
    os: VALID_OS.has(body.os ?? "") ? body.os : "unknown",
    arch: VALID_ARCH.has(body.arch ?? "") ? body.arch : "unknown",
    success: Boolean(body.success ?? false),
    tokens_per_sec_generation:
      typeof body.tokens_per_sec_generation === "number"
        ? Math.max(0, body.tokens_per_sec_generation)
        : null,
    tokens_per_sec_prefill:
      typeof body.tokens_per_sec_prefill === "number"
        ? Math.max(0, body.tokens_per_sec_prefill)
        : null,
    peak_vram_mb:
      typeof body.peak_vram_mb === "number"
        ? Math.max(0, Math.floor(body.peak_vram_mb))
        : null,
    duration_seconds:
      typeof body.duration_seconds === "number"
        ? Math.max(0, body.duration_seconds)
        : null,
    recorded_at: new Date().toISOString(),
  };

  // P-03/F-04: Use module-scoped admin singleton — fails fast if service key absent
  const supabase = getSupabaseAdmin();
  if (!supabase) {
    // Gracefully accept but don't store — CLI must not fail because of this
    return NextResponse.json({ ok: true, stored: false }, { status: 200 });
  }

  // Find the recipe UUID — needed to insert into telemetry_events
  const { data: recipeRow, error: lookupError } = await supabase
    .from("recipes")
    .select("id")
    .eq("creator", author)
    .eq("name", recipeName)
    .maybeSingle();

  if (lookupError || !recipeRow) {
    // Recipe not found — don't error the CLI, just silently discard
    return NextResponse.json({ ok: true, stored: false }, { status: 200 });
  }

  // P-04: Single INSERT into telemetry_events — no read-modify-write race.
  // This replaces the previous SELECT + UPDATE on compat_builds JSONB.
  //
  // ⚠️ ACTION REQUIRED: Run the following SQL in Supabase to create this table:
  //
  //   CREATE TABLE IF NOT EXISTS telemetry_events (
  //     id                        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  //     recipe_id                 uuid REFERENCES recipes(id) ON DELETE CASCADE,
  //     recorded_at               timestamptz NOT NULL DEFAULT now(),
  //     cli_version               text,
  //     os                        text,
  //     arch                      text,
  //     success                   boolean,
  //     tokens_per_sec_generation float8,
  //     tokens_per_sec_prefill    float8,
  //     peak_vram_mb              bigint,
  //     duration_seconds          float8
  //   );
  //   CREATE INDEX idx_telemetry_recipe_id ON telemetry_events (recipe_id);
  //   CREATE INDEX idx_telemetry_recorded_at ON telemetry_events (recorded_at DESC);
  //
  // Until this table exists, we fall back to the compat_builds JSONB approach below.

  // Try telemetry_events table first (P-04)
  const { error: insertError } = await supabase
    .from("telemetry_events")
    .insert({
      recipe_id: recipeRow.id,
      ...benchmarkEntry,
    });

  if (!insertError) {
    return NextResponse.json({ ok: true, stored: true }, { status: 200 });
  }

  // Fallback: compat_builds JSONB (used until telemetry_events table is created)
  // This preserves backward compatibility during the migration window.
  const { data: recipeWithBuilds, error: buildsLookupError } = await supabase
    .from("recipes")
    .select("id, compat_builds")
    .eq("id", recipeRow.id)
    .maybeSingle();

  if (buildsLookupError || !recipeWithBuilds) {
    return NextResponse.json({ ok: true, stored: false }, { status: 200 });
  }

  const existing: any[] = Array.isArray(recipeWithBuilds.compat_builds)
    ? recipeWithBuilds.compat_builds
    : [];
  const updated = [...existing, benchmarkEntry].slice(-1000);

  const { error: updateError } = await supabase
    .from("recipes")
    .update({ compat_builds: updated })
    .eq("id", recipeRow.id);

  if (updateError) {
    console.error("[API] /api/telemetry update error:", updateError);
    return NextResponse.json({ ok: true, stored: false }, { status: 200 });
  }

  return NextResponse.json({ ok: true, stored: true }, { status: 200 });
}

// The CLI sends a preflight OPTIONS before POST
export async function OPTIONS() {
  return new Response(null, {
    status: 204,
    headers: {
      // F-11: Restricted CORS — wildcard allowed any webpage to fake benchmarks
      "Access-Control-Allow-Origin": CORS_ORIGIN,
      "Access-Control-Allow-Methods": "POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type, User-Agent",
      "Vary": "Origin",
    },
  });
}
