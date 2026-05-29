import { NextRequest, NextResponse } from "next/server";
import { getSupabaseAdmin } from "@/lib/supabase-server";

// Generates a human-readable code like ABCD-1234.
// Excludes visually ambiguous characters: 0, O, 1, I.
function generateUserCode(): string {
  const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  const seg = () =>
    Array.from(
      { length: 4 },
      () => chars[Math.floor(Math.random() * chars.length)]
    ).join("");
  return `${seg()}-${seg()}`;
}

export async function POST(_req: NextRequest) {
  const supabase = getSupabaseAdmin();
  if (!supabase) {
    return NextResponse.json(
      { error: "Service unavailable — auth not configured" },
      { status: 503 }
    );
  }

  const deviceCode = crypto.randomUUID();
  const userCode = generateUserCode();
  // Expires in 15 minutes
  const expiresAt = new Date(Date.now() + 15 * 60 * 1000).toISOString();

  const { error } = await supabase.from("device_codes").insert({
    device_code: deviceCode,
    user_code: userCode,
    expires_at: expiresAt,
  });

  if (error) {
    console.error("[auth/device] Failed to create device code:", error);
    return NextResponse.json(
      { error: "Failed to start device flow" },
      { status: 500 }
    );
  }

  return NextResponse.json({
    device_code: deviceCode,
    user_code: userCode,
    verification_url: "https://bloc-hub.com/auth/device",
    expires_in: 900, // seconds
  });
}

export async function OPTIONS() {
  return new Response(null, {
    status: 204,
    headers: {
      // CLI is a Go HTTP client — CORS doesn't apply, but keep for browser callers
      "Access-Control-Allow-Origin": "https://bloc-hub.com",
      "Access-Control-Allow-Methods": "POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
    },
  });
}
