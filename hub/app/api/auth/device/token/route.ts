import { NextRequest, NextResponse } from "next/server";
import { getSupabaseAdmin } from "@/lib/supabase-server";

// CLI polls this every 5 seconds with its device_code.
// Responds with: "pending" | "expired" | "authorized" (+ token + username)
export async function POST(req: NextRequest) {
  const supabase = getSupabaseAdmin();
  if (!supabase) {
    return NextResponse.json(
      { error: "Service unavailable" },
      { status: 503 }
    );
  }

  let body: { device_code?: string };
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "Invalid JSON" }, { status: 400 });
  }

  const deviceCode = body.device_code?.trim();
  if (!deviceCode || typeof deviceCode !== "string" || deviceCode.length > 64) {
    return NextResponse.json({ error: "Invalid device_code" }, { status: 400 });
  }

  const { data, error } = await supabase
    .from("device_codes")
    .select("authorized, access_token, username, expires_at")
    .eq("device_code", deviceCode)
    .maybeSingle();

  if (error) {
    console.error("[auth/device/token] lookup error:", error);
    return NextResponse.json({ error: "Database error" }, { status: 500 });
  }

  // Not found → treat as expired (either never existed or already consumed)
  if (!data) {
    return NextResponse.json({ status: "expired" });
  }

  // Check expiry
  if (new Date(data.expires_at) < new Date()) {
    // Clean up the stale row
    await supabase.from("device_codes").delete().eq("device_code", deviceCode);
    return NextResponse.json({ status: "expired" });
  }

  // Still waiting for user to authorize in browser
  if (!data.authorized) {
    return NextResponse.json({ status: "pending" });
  }

  // Authorized — return token and clean up the row (one-time use)
  await supabase.from("device_codes").delete().eq("device_code", deviceCode);

  return NextResponse.json({
    status: "authorized",
    token: data.access_token,
    username: data.username,
  });
}

export async function OPTIONS() {
  return new Response(null, {
    status: 204,
    headers: {
      "Access-Control-Allow-Origin": "https://bloc-hub.com",
      "Access-Control-Allow-Methods": "POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
    },
  });
}
