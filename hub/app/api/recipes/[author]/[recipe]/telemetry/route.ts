import { NextRequest, NextResponse } from "next/server";
import { getSupabaseAnon } from "@/lib/supabase-server";

const CORS_ORIGIN = "https://bloc-hub.com";
const SAFE_SEGMENT_RE = /^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,99}$/;

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ author: string; recipe: string }> }
) {
  const { author, recipe } = await params;

  if (!author || !recipe || !SAFE_SEGMENT_RE.test(author) || !SAFE_SEGMENT_RE.test(recipe)) {
    return NextResponse.json(
      { error: "Invalid recipe identifier" },
      { status: 400 }
    );
  }

  const supabase = getSupabaseAnon();
  if (!supabase) {
    return NextResponse.json(
      { error: "Service unavailable — database not configured" },
      { status: 503 }
    );
  }

  // 1. Get recipe UUID
  const { data: recipeRow, error: lookupError } = await supabase
    .from("recipes")
    .select("id")
    .eq("creator", author)
    .eq("name", recipe)
    .maybeSingle();

  if (lookupError) {
    console.error("[API] /api/recipes/[author]/[recipe]/telemetry lookup error:", lookupError);
    return NextResponse.json(
      { error: "Database error" },
      { status: 500 }
    );
  }

  if (!recipeRow) {
    return NextResponse.json(
      { error: "Recipe not found" },
      { status: 404 }
    );
  }

  // 2. Fetch the latest 1000 telemetry events
  const { data: events, error: telemetryError } = await supabase
    .from("telemetry_events")
    .select("os, arch, success, tokens_per_sec_generation, tokens_per_sec_prefill, recorded_at")
    .eq("recipe_id", recipeRow.id)
    .order("recorded_at", { ascending: false })
    .limit(1000);

  if (telemetryError) {
    console.error("[API] /api/recipes/[author]/[recipe]/telemetry events error:", telemetryError);
    return NextResponse.json(
      { error: "Database error fetching telemetry" },
      { status: 500 }
    );
  }

  // 3. Aggregate data
  let totalRuns = 0;
  let successRuns = 0;

  interface PlatformStats {
    runs: number;
    successRuns: number;
    tokensSum: number;
    tokensCount: number;
    prefillSum: number;
    prefillCount: number;
    latestRun: string;
  }

  const groups: Record<string, PlatformStats> = {};

  if (events) {
    for (const event of events) {
      totalRuns++;
      if (event.success) {
        successRuns++;
      }

      const platform = `${event.os || "unknown"}/${event.arch || "unknown"}`;
      if (!groups[platform]) {
        groups[platform] = {
          runs: 0,
          successRuns: 0,
          tokensSum: 0,
          tokensCount: 0,
          prefillSum: 0,
          prefillCount: 0,
          latestRun: event.recorded_at,
        };
      }

      const g = groups[platform];
      g.runs++;
      if (event.success) {
        g.successRuns++;
      }

      if (event.tokens_per_sec_generation != null) {
        g.tokensSum += Number(event.tokens_per_sec_generation);
        g.tokensCount++;
      }
      if (event.tokens_per_sec_prefill != null) {
        g.prefillSum += Number(event.tokens_per_sec_prefill);
        g.prefillCount++;
      }

      if (new Date(event.recorded_at) > new Date(g.latestRun)) {
        g.latestRun = event.recorded_at;
      }
    }
  }

  const benchmarks = Object.entries(groups).map(([platform, g]) => ({
    platform,
    runs: g.runs,
    avg_tokens_per_sec: g.tokensCount > 0 ? parseFloat((g.tokensSum / g.tokensCount).toFixed(1)) : 0,
    avg_prefill_tokens_per_sec: g.prefillCount > 0 ? parseFloat((g.prefillSum / g.prefillCount).toFixed(1)) : 0,
    latest_run: g.latestRun,
  }));

  const successRate = totalRuns > 0 ? parseFloat((successRuns / totalRuns).toFixed(2)) : 0;

  return NextResponse.json(
    {
      total_runs: totalRuns,
      success_rate: successRate,
      benchmarks,
    },
    {
      status: 200,
      headers: {
        "Access-Control-Allow-Origin": CORS_ORIGIN,
        "Access-Control-Allow-Methods": "GET",
        "Cache-Control": "public, s-maxage=120, stale-while-revalidate=600",
        "Vary": "Origin",
      },
    }
  );
}

export async function OPTIONS() {
  return new Response(null, {
    status: 204,
    headers: {
      "Access-Control-Allow-Origin": CORS_ORIGIN,
      "Access-Control-Allow-Methods": "GET, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type, User-Agent",
      "Vary": "Origin",
    },
  });
}
