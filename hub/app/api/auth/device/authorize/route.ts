import { NextRequest, NextResponse } from "next/server";
import { getSupabaseAdmin } from "@/lib/supabase-server";

// Called by the browser device page after the user signs in and submits their user_code.
// Validates the user's session, looks up the device row by user_code, and marks it authorized.
export async function POST(req: NextRequest) {
  const supabase = getSupabaseAdmin();
  if (!supabase) {
    return NextResponse.json(
      { error: "Service unavailable" },
      { status: 503 }
    );
  }

  // Verify the caller is authenticated — browser sends its Supabase JWT
  const authHeader = req.headers.get("authorization");
  const userToken = authHeader?.startsWith("Bearer ")
    ? authHeader.slice(7).trim()
    : null;

  if (!userToken) {
    return NextResponse.json(
      { error: "You must be signed in to authorize the CLI" },
      { status: 401 }
    );
  }

  // Verify the token with Supabase auth
  const {
    data: { user },
    error: userError,
  } = await supabase.auth.getUser(userToken);

  if (userError || !user) {
    return NextResponse.json(
      { error: "Session invalid or expired — please sign in again" },
      { status: 401 }
    );
  }

  // Resolve their bloc username from profiles
  const { data: profile } = await supabase
    .from("profiles")
    .select("username")
    .eq("auth_id", user.id)
    .maybeSingle();

  // Fall back to GitHub handle if profile not set up yet
  const username =
    profile?.username ||
    user.user_metadata?.user_name ||
    user.user_metadata?.preferred_username ||
    "";

  // Parse the user_code from the request body
  let body: { user_code?: string };
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "Invalid JSON body" }, { status: 400 });
  }

  const userCode = body.user_code?.trim().toUpperCase();
  if (!userCode || !/^[A-Z0-9]{4}-[A-Z0-9]{4}$/.test(userCode)) {
    return NextResponse.json(
      { error: "Invalid code — enter the 8-character code shown in your terminal" },
      { status: 400 }
    );
  }

  // Look up the device_code row by user_code
  const { data: row, error: lookupError } = await supabase
    .from("device_codes")
    .select("device_code, expires_at, authorized")
    .eq("user_code", userCode)
    .maybeSingle();

  if (lookupError) {
    console.error("[auth/device/authorize] lookup error:", lookupError);
    return NextResponse.json({ error: "Database error" }, { status: 500 });
  }

  if (!row) {
    return NextResponse.json(
      { error: "Code not found — check for typos or run 'bloc login' again" },
      { status: 404 }
    );
  }

  if (new Date(row.expires_at) < new Date()) {
    await supabase.from("device_codes").delete().eq("device_code", row.device_code);
    return NextResponse.json(
      { error: "Code expired — run 'bloc login' again to get a new one" },
      { status: 410 }
    );
  }

  if (row.authorized) {
    return NextResponse.json(
      { error: "Code already used" },
      { status: 409 }
    );
  }

  // Mark the row as authorized and store the access token + username
  const { error: updateError } = await supabase
    .from("device_codes")
    .update({
      authorized: true,
      access_token: userToken,
      username: username,
    })
    .eq("device_code", row.device_code);

  if (updateError) {
    console.error("[auth/device/authorize] update error:", updateError);
    return NextResponse.json(
      { error: "Failed to authorize — please try again" },
      { status: 500 }
    );
  }

  return NextResponse.json({ ok: true, username });
}
