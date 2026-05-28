import Link from "next/link";
import { notFound } from "next/navigation";
import { cache } from "react";
import { registryRecipes, Recipe } from "@/lib/registry-data";
import type { Metadata } from "next";
import { supabase } from "@/lib/supabase";

// Custom component to render the copy deployment console
import CopyCommandBox from "./CopyCommandBox";

function RecipeHighlight({ children }: { children: React.ReactNode }) {
  return (
    <span 
      className="inline text-black dark:text-white px-2 box-decoration-clone"
      style={{
        fontFamily: 'inherit',
        background: 'linear-gradient(to bottom, transparent 15%, #2563EB 15%, #2563EB 82%, transparent 82%)'
      }}
    >
      {children}
    </span>
  );
}

interface PageProps {
  params: Promise<{
    author: string;
    recipe: string;
  }>;
}

// React cache() deduplicates identical calls within the same request
// (e.g. the main page render + generateMetadata both call this)
const getRecipe = cache(async function getRecipe(
  author: string,
  name: string
): Promise<Recipe | null> {
  const recipeId = `${author}/${name}`;
  const mockRecipe = registryRecipes.find((r) => r.id === recipeId);
  if (mockRecipe) return mockRecipe;

  if (!supabase) return null;

  try {
    const { data, error } = await supabase
      .from("recipes")
      .select(
        "id, name, creator, description, base_model, min_vram, target_platform, yaml_content, compat_builds"
      )
      .eq("creator", author)
      .eq("name", name)
      .limit(1)
      .maybeSingle();

    if (error) throw error;

    if (data) {
      const quantMatch = data.yaml_content?.match(
        /quantization:\s*(?:"([^"]+)"|'([^"]+)'|([a-zA-Z0-9_-]+))/
      );
      const quantization = quantMatch
        ? quantMatch[1] || quantMatch[2] || quantMatch[3]
        : "Q4_K_M";

      return {
        id: `${data.creator}/${data.name}`,
        name: data.name,
        creator: data.creator,
        description: data.description || "",
        baseModel: data.base_model,
        engine: "llama.cpp",
        quantization: quantization,
        hardware: {
          minVram: data.min_vram,
          targetPlatform: data.target_platform as any,
        },
        verified: "none",
        telemetry: {
          runs: data.compat_builds?.length || 0,
          benchmarks: [],
        },
      };
    }
  } catch (e) {
    console.error("Error loading recipe details from Supabase:", e);
  }

  return null;
});

export default async function RecipeDetailPage(props: PageProps) {
  const params = await props.params;
  const recipe = await getRecipe(params.author, params.recipe);

  if (!recipe) {
    notFound();
  }

  return (
    <div className="max-w-4xl w-full mx-auto px-6 py-16 select-none">
      
      {/* Back to Registry */}
      <Link href="/registry" className="inline-flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors mb-12">
        <svg width="8" height="8" viewBox="0 0 10 10" fill="none" xmlns="http://www.w3.org/2000/svg" className="transform rotate-180">
          <path d="M1 9L9 1M9 1H1M9 1V9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
        Back to registry
      </Link>

      {/* Header Profile card */}
      <div className="relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-8 rounded-none mb-12">
        {/* SVG Corner L-Brackets */}
        <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white pointer-events-none">
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none">
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none">
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>

        <div className="flex items-center gap-2 mb-4">
          <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400">
            {recipe.hardware.targetPlatform.toUpperCase()}
          </span>
          <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 bg-zinc-200 dark:bg-zinc-900 text-zinc-850 dark:text-zinc-200">
            {recipe.hardware.minVram} VRAM
          </span>
          {recipe.verified !== "none" && (
            <span className={`font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 font-bold ${
              recipe.verified === "publisher"
                ? "border border-blue-500/20 bg-blue-500/10 text-blue-600 dark:text-blue-400"
                : "border border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
            }`}>
              {recipe.verified === "publisher" ? "Verified Org" : "Community Verified"}
            </span>
          )}
        </div>

        <h1 className="text-3xl md:text-4xl font-semibold tracking-tight font-switzer text-black dark:text-white mb-4 leading-tight">
          <Link href={`/users/${recipe.creator}`} className="hover:text-blue-600 dark:hover:text-blue-400 underline decoration-dotted transition-colors">
            {recipe.creator}
          </Link>/<span className="text-blue-600 dark:text-blue-400">{recipe.name}</span>
        </h1>
        <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-3xl leading-relaxed mb-6">
          {recipe.description}
        </p>

        {/* Copy Deploy box */}
        <CopyCommandBox recipeId={recipe.id} />
      </div>

      {/* Details Grid (Telemetry vs Configuration Table) */}
      <div className="grid grid-cols-1 md:grid-cols-12 gap-8 items-start w-full">
        
        {/* Left Column: Spec Sheet (7/12) */}
        <div className="md:col-span-7 flex flex-col gap-6">
          <h2 className="text-xl font-semibold font-switzer tracking-tight text-black dark:text-white border-b border-zinc-200 dark:border-zinc-800 pb-2">
            Configuration Specifications
          </h2>
          
          <div className="border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 rounded-none overflow-hidden">
            <table className="w-full text-left font-mono text-[11px] border-collapse">
              <tbody>
                <tr className="border-b border-zinc-200 dark:border-zinc-800">
                  <td className="px-4 py-3 font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wider bg-zinc-200/20 dark:bg-zinc-950/20 w-1/3">Base Model</td>
                  <td className="px-4 py-3 text-zinc-800 dark:text-zinc-200">{recipe.baseModel}</td>
                </tr>
                <tr className="border-b border-zinc-200 dark:border-zinc-800">
                  <td className="px-4 py-3 font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wider bg-zinc-200/20 dark:bg-zinc-950/20">Engine</td>
                  <td className="px-4 py-3 text-zinc-800 dark:text-zinc-200">{recipe.engine}</td>
                </tr>
                <tr className="border-b border-zinc-200 dark:border-zinc-800">
                  <td className="px-4 py-3 font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wider bg-zinc-200/20 dark:bg-zinc-950/20">Quantization</td>
                  <td className="px-4 py-3 text-zinc-800 dark:text-zinc-200">{recipe.quantization}</td>
                </tr>
                <tr className="border-b border-zinc-200 dark:border-zinc-800">
                  <td className="px-4 py-3 font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wider bg-zinc-200/20 dark:bg-zinc-950/20">Platform</td>
                  <td className="px-4 py-3 text-zinc-800 dark:text-zinc-200 uppercase">{recipe.hardware.targetPlatform}</td>
                </tr>
                <tr>
                  <td className="px-4 py-3 font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wider bg-zinc-200/20 dark:bg-zinc-950/20">VRAM Required</td>
                  <td className="px-4 py-3 text-zinc-800 dark:text-zinc-200">{recipe.hardware.minVram} minimum</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        {/* Right Column: Telemetry Performance (5/12) */}
        <div className="md:col-span-5 flex flex-col gap-6">
          <h2 className="text-xl font-semibold font-switzer tracking-tight text-black dark:text-white border-b border-zinc-200 dark:border-zinc-800 pb-2">
            Telemetry Benchmarks
          </h2>
          
          <div className="border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 p-6 rounded-none">
            <div className="flex items-center justify-between border-b border-zinc-200 dark:border-zinc-800 pb-3 mb-4 font-mono text-[10px] font-bold text-zinc-500 uppercase tracking-wider">
              <span>Verified GPU Node</span>
              <span>Avg Speed</span>
            </div>
            
            <div className="flex flex-col gap-4 font-mono text-xs">
              {recipe.telemetry.benchmarks.map((bench, idx) => (
                <div key={idx} className="flex justify-between items-center gap-4">
                  <div className="flex flex-col">
                    <span className="text-zinc-800 dark:text-zinc-200 font-bold">{bench.gpu}</span>
                    <span className="text-[9px] text-zinc-400 dark:text-zinc-500">({bench.runs} telemetry runs)</span>
                  </div>
                  <span className="text-blue-600 dark:text-blue-400 font-bold shrink-0">{bench.tokensPerSec} t/s</span>
                </div>
              ))}
            </div>
          </div>
        </div>

      </div>

    </div>
  );
}

// Generate static params for optimal SSG compilation
export async function generateStaticParams() {
  return registryRecipes.map((recipe) => {
    const parts = recipe.id.split("/");
    return {
      author: parts[0],
      recipe: parts[1],
    };
  });
}

// SEO Metadata mapping
export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const params = await props.params;
  const recipe = await getRecipe(params.author, params.recipe);
  
  if (!recipe) {
    return { title: "Recipe Not Found" };
  }

  return {
    title: `${recipe.creator}/${recipe.name} Local AI Recipe - Bloc Hub`,
    description: `Deploy ${recipe.name} locally in one command. Optimized ${recipe.quantization} configuration for ${recipe.hardware.minVram} VRAM systems.`,
  };
}
