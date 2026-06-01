import { Suspense } from "react";
import { registryRecipes, Recipe } from "@/lib/registry-data";
import { getSupabaseAnon } from "@/lib/supabase-server";
import RegistryClient from "./RegistryClient";
import type { Metadata } from "next";

export const revalidate = 60; // Revalidate registry cache every 60 seconds

export const metadata: Metadata = {
  title: "Model Registry - Bloc Hub",
  description: "Discover, configure, and pull optimized local AI model recipes submitted by the community. Run them instantly on your local hardware.",
};

async function getDbRecipes(): Promise<Recipe[]> {
  const supabase = getSupabaseAnon();
  if (!supabase) return [];

  try {
    const { data, error } = await supabase
      .from("recipes")
      .select("id, name, creator, description, base_model, min_vram, target_platform, yaml_content, compat_builds, created_at, telemetry_events(count)")
      .order("created_at", { ascending: false })
      .limit(500);

    if (error) throw error;
    if (!data) return [];

    return data.map((row: any) => {
      const quantMatch = row.yaml_content?.match(/quantization:\s*(?:"([^"]+)"|'([^"]+)'|([a-zA-Z0-9_-]+))/);
      const quantization = quantMatch ? (quantMatch[1] || quantMatch[2] || quantMatch[3]) : "Q4_K_M";
      
      return {
        id: `${row.creator}/${row.name}`,
        name: row.name,
        creator: row.creator,
        description: row.description || "",
        baseModel: row.base_model,
        engine: "llama.cpp",
        quantization: quantization,
        hardware: {
          minVram: row.min_vram,
          targetPlatform: row.target_platform as any
        },
        verified: "none" as const,
        telemetry: {
          runs: row.telemetry_events?.[0]?.count || 0,
          benchmarks: []
        }
      };
    });
  } catch (e) {
    console.error("Error loading recipes on server:", e);
    return [];
  }
}

export default async function RegistryPage() {
  const dbRecipes = await getDbRecipes();
  
  // Combine static mock data and dynamic Supabase recipes on the server
  const allRecipes = [
    ...registryRecipes,
    ...dbRecipes.filter(dbR => !registryRecipes.some(mockR => mockR.id === dbR.id))
  ];

  return (
    <Suspense fallback={<div className="p-12 text-center text-sm font-mono text-zinc-500">Loading Registry...</div>}>
      <RegistryClient initialRecipes={allRecipes} />
    </Suspense>
  );
}
