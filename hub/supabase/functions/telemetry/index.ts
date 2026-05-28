// Location: hub/supabase/functions/telemetry/index.ts
import { serve } from "https://deno.land/std@0.168.0/http/server.ts";
import { createClient } from "https://esm.sh/@supabase/supabase-js@2.21.0";

const corsHeaders = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers": "authorization, x-client-info, apikey, content-type",
  "Access-Control-Allow-Methods": "POST, OPTIONS",
};

serve(async (req) => {
  // 1. Handle CORS Pre-flight Options Request
  if (req.method === "OPTIONS") {
    return new Response("ok", { headers: corsHeaders });
  }

  if (req.method !== "POST") {
    return new Response(JSON.stringify({ error: "Method not allowed" }), {
      status: 405,
      headers: { ...corsHeaders, "Content-Type": "application/json" },
    });
  }

  try {
    const payload = await req.json();
    const {
      recipe_identifier, // e.g. "arnav080/qwen3-30b-moe-8gb-cpu-offload"
      success,
      tokens_per_sec,
      peak_vram_mb,
      os,
      arch,
      llama_build,
    } = payload;

    // Validate minimum required fields
    if (!recipe_identifier || success === undefined) {
      return new Response(JSON.stringify({ error: "Missing required fields: recipe_identifier, success" }), {
        status: 400,
        headers: { ...corsHeaders, "Content-Type": "application/json" },
      });
    }

    // Split "creator/recipe_name" identifier
    const parts = recipe_identifier.split("/");
    if (parts.length !== 2) {
      return new Response(JSON.stringify({ error: "Invalid recipe identifier format. Expected: 'creator/name'" }), {
        status: 400,
        headers: { ...corsHeaders, "Content-Type": "application/json" },
      });
    }
    const [creator, name] = parts;

    // Initialize Supabase Client with Service Role Key (bypasses RLS to insert telemetry securely)
    const supabaseClient = createClient(
      Deno.env.get("SUPABASE_URL") ?? "",
      Deno.env.get("SUPABASE_SERVICE_ROLE_KEY") ?? "",
      { auth: { persistSession: false } }
    );

    // 2. Resolve Recipe ID from Creator and Name
    const { data: recipe, error: recipeErr } = await supabaseClient
      .from("recipes")
      .select("id, compat_builds")
      .eq("creator", creator)
      .eq("name", name)
      .single();

    if (recipeErr || !recipe) {
      return new Response(JSON.stringify({ error: `Recipe '${recipe_identifier}' not found.` }), {
        status: 404,
        headers: { ...corsHeaders, "Content-Type": "application/json" },
      });
    }

    // 3. Write Sanitized Metrics into public.telemetry
    const { error: telemetryErr } = await supabaseClient
      .from("telemetry")
      .insert({
        recipe_id: recipe.id,
        success,
        tokens_per_sec: tokens_per_sec || null,
        peak_vram_mb: peak_vram_mb || null,
        os: os || null,
        arch: arch || null,
        llama_build: llama_build || null,
      });

    if (telemetryErr) {
      console.error("Telemetry insert failed:", telemetryErr);
      throw telemetryErr;
    }

    // 4. Update compat_builds Array on Recipes Table
    if (success && llama_build) {
      const builds = Array.isArray(recipe.compat_builds) ? recipe.compat_builds : [];
      if (!builds.includes(llama_build)) {
        const updatedBuilds = [...builds, llama_build];
        await supabaseClient
          .from("recipes")
          .update({ compat_builds: updatedBuilds })
          .eq("id", recipe.id);
      }
    }

    return new Response(JSON.stringify({ success: true, message: "Telemetry recorded successfully." }), {
      status: 200,
      headers: { ...corsHeaders, "Content-Type": "application/json" },
    });

  } catch (error) {
    console.error("Telemetry Endpoint Error:", error);
    // Standard telemetry resilience: Return 200 so the client CLI never breaks or alerts the user
    return new Response(JSON.stringify({ success: false, message: "Telemetry silently absorbed." }), {
      status: 200,
      headers: { ...corsHeaders, "Content-Type": "application/json" },
    });
  }
});
