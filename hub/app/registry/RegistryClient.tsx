"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import ShortcutButton from "@/components/ShortcutButton";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Download, Star } from "lucide-react";
import { useAuth } from "@/context/AuthContext";
import { supabase } from "@/lib/supabase";
import { toast } from "sonner";
import { Recipe } from "@/lib/registry-data";

function HighlightTitle({ children }: { children: React.ReactNode }) {
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

// Helpers for model grouping
function getModelFamily(baseModel: string) {
  if (baseModel.includes("Qwen2.5-7B")) return "Qwen 2.5 7B";
  if (baseModel.includes("Qwen2.5-Coder-32B")) return "Qwen 2.5 Coder 32B";
  if (baseModel.includes("DeepSeek-R1-Distill-Llama-8B")) return "DeepSeek R1 8B";
  if (baseModel.includes("Mistral-7B")) return "Mistral 7B";
  if (baseModel.includes("gemma-2-9b")) return "Gemma 2 9B";
  return baseModel;
}

function getModelBrand(baseModel: string) {
  if (baseModel.toLowerCase().includes("qwen")) return "Qwen";
  if (baseModel.toLowerCase().includes("deepseek")) return "DeepSeek";
  if (baseModel.toLowerCase().includes("mistral")) return "Mistral";
  if (baseModel.toLowerCase().includes("gemma")) return "Gemma";
  return "Other";
}

interface RegistryClientProps {
  initialRecipes: Recipe[];
}

export default function RegistryClient({ initialRecipes }: RegistryClientProps) {
  const router = useRouter();
  const { user } = useAuth();

  // Starred Recipes State
  const [starredRecipeIds, setStarredRecipeIds] = useState<string[]>([]);
  
  // M1: Rate limiting/spam protection for Star toggles
  const pendingStarsRef = useRef<Set<string>>(new Set());

  // Load stars dynamically from Supabase
  useEffect(() => {
    let active = true;
    async function loadStars() {
      if (!supabase || !user) {
        setStarredRecipeIds([]);
        return;
      }
      try {
        const { data, error } = await supabase
          .from("stars")
          .select("recipe_id")
          .eq("user_id", user.id);
        if (error) throw error;
        if (active) {
          setStarredRecipeIds((data || []).map((row: any) => row.recipe_id));
        }
      } catch (err) {
        console.error("Error loading stars:", err);
      }
    }
    loadStars();
    return () => {
      active = false;
    };
  }, [user]);

  const toggleStar = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    if (!supabase || !user) {
      toast.error("Please sign in to star recipes.");
      return;
    }

    if (pendingStarsRef.current.has(id)) return;
    pendingStarsRef.current.add(id);

    const isCurrentlyStarred = starredRecipeIds.includes(id);
    const nextState = !isCurrentlyStarred;

    // Optimistic UI update
    setStarredRecipeIds(prev =>
      nextState ? [...prev, id] : prev.filter(item => item !== id)
    );

    try {
      if (nextState) {
        const { error } = await supabase
          .from("stars")
          .insert({
            user_id: user.id,
            recipe_id: id
          });
        if (error) throw error;
        toast.success("Saved to Starred configs");
      } else {
        const { error } = await supabase
          .from("stars")
          .delete()
          .eq("user_id", user.id)
          .eq("recipe_id", id);
        if (error) throw error;
        toast.info("Removed from Starred configs");
      }
    } catch (err) {
      console.error("Error toggling star:", err);
      toast.error("Failed to update star. Please try again.");
      
      // Rollback optimistic state
      setStarredRecipeIds(prev =>
        isCurrentlyStarred ? [...prev, id] : prev.filter(item => item !== id)
      );
    } finally {
      pendingStarsRef.current.delete(id);
    }
  };

  // Filter States
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedSearchQuery, setDebouncedSearchQuery] = useState("");

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedSearchQuery(searchQuery);
    }, 200);
    return () => clearTimeout(handler);
  }, [searchQuery]);

  const [selectedVram, setSelectedVram] = useState<string[]>([]);
  const [selectedPlatform, setSelectedPlatform] = useState<string[]>([]);
  const [selectedVerification, setSelectedVerification] = useState<string[]>([]);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  
  // View Modes: 'default' | 'model' | 'gpu'
  const [viewMode, setViewMode] = useState<"default" | "model" | "gpu">("default");
  const [selectedModel, setSelectedModel] = useState<string | null>(null);
  const [selectedGpu, setSelectedGpu] = useState<string | null>(null);

  // Keyboard Shortcuts for 1 and 2
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        document.activeElement &&
        (document.activeElement.tagName === "INPUT" ||
          document.activeElement.tagName === "TEXTAREA" ||
          (document.activeElement as HTMLElement).isContentEditable)
      ) {
        return;
      }

      if (e.key === "1") {
        e.preventDefault();
        setViewMode((prev) => {
          const next = prev === "model" ? "default" : "model";
          setSelectedModel(null);
          setSelectedGpu(null);
          return next;
        });
      } else if (e.key === "2") {
        e.preventDefault();
        setViewMode((prev) => {
          const next = prev === "gpu" ? "default" : "gpu";
          setSelectedModel(null);
          setSelectedGpu(null);
          return next;
        });
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Clipboard Copy State
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const copyTimeoutRef = useRef<any>(null);

  useEffect(() => {
    return () => {
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    };
  }, []);

  const handleCopy = (id: string) => {
    const command = `bloc deploy ${id}`;
    navigator.clipboard.writeText(command);
    setCopiedId(id);
    if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    copyTimeoutRef.current = setTimeout(() => setCopiedId(null), 1500);
  };

  // Filter Logic
  const filteredRecipes = useMemo(() => {
    return initialRecipes.filter((recipe) => {
      const matchesSearch = 
        recipe.name.toLowerCase().includes(debouncedSearchQuery.toLowerCase()) ||
        recipe.creator.toLowerCase().includes(debouncedSearchQuery.toLowerCase()) ||
        recipe.description.toLowerCase().includes(debouncedSearchQuery.toLowerCase()) ||
        recipe.baseModel.toLowerCase().includes(debouncedSearchQuery.toLowerCase());

      const matchesVram = 
        selectedVram.length === 0 || 
        selectedVram.includes(recipe.hardware.minVram);

      const matchesPlatform = 
        selectedPlatform.length === 0 || 
        selectedPlatform.includes(recipe.hardware.targetPlatform.toUpperCase());

      const matchesVerification = 
        selectedVerification.length === 0 || 
        selectedVerification.includes(recipe.verified);

      const matchesModelSelection = 
        !selectedModel || 
        getModelFamily(recipe.baseModel) === selectedModel;

      const matchesGpuSelection = 
        !selectedGpu || 
        recipe.telemetry.benchmarks.some((b) => b.gpu.toLowerCase().includes(selectedGpu.toLowerCase()));

      return matchesSearch && matchesVram && matchesPlatform && matchesVerification && matchesModelSelection && matchesGpuSelection;
    });
  }, [initialRecipes, debouncedSearchQuery, selectedVram, selectedPlatform, selectedVerification, selectedModel, selectedGpu]);

  const toggleFilter = (list: string[], setList: (v: string[]) => void, item: string) => {
    if (list.includes(item)) {
      setList(list.filter((x) => x !== item));
    } else {
      setList([...list, item]);
    }
  };

  // Extract Unique Models & Recipe count for model view mode
  const uniqueModels = useMemo(() => {
    return Array.from(
      new Set(initialRecipes.map((r) => getModelFamily(r.baseModel)))
    ).map((modelName) => {
      const recipesCount = initialRecipes.filter((r) => getModelFamily(r.baseModel) === modelName).length;
      const sampleRecipe = initialRecipes.find((r) => getModelFamily(r.baseModel) === modelName);
      const brand = sampleRecipe ? getModelBrand(sampleRecipe.baseModel) : "Other";
      return { name: modelName, brand, count: recipesCount };
    });
  }, [initialRecipes]);

  // Extract Unique GPUs & Recipe count for GPU view mode
  const uniqueGpus = useMemo(() => {
    return Array.from(
      new Set(
        initialRecipes.flatMap((r) => r.telemetry.benchmarks.map((b) => b.gpu))
      )
    ).map((gpuName) => {
      const recipesCount = initialRecipes.filter((r) => 
        r.telemetry.benchmarks.some((b) => b.gpu === gpuName)
      ).length;
      
      const runsForGpu = initialRecipes.flatMap((r) => 
        r.telemetry.benchmarks.filter((b) => b.gpu === gpuName)
      );
      const avgTps = runsForGpu.length > 0 
        ? Math.round(runsForGpu.reduce((acc, curr) => acc + curr.tokensPerSec, 0) / runsForGpu.length)
        : 0;

      return { name: gpuName, count: recipesCount, speed: avgTps };
    });
  }, [initialRecipes]);

  return (
    <div className="max-w-6xl w-full mx-auto px-6 py-12">
      
      {/* Title */}
      <div className="w-full text-left mt-12 mb-16">
        <h1 className="text-4xl md:text-5xl font-medium tracking-tight font-switzer text-black dark:text-white mb-6">
          <HighlightTitle>Model Registry</HighlightTitle>
        </h1>
        <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-2xl leading-relaxed">
          Discover, configure, and pull optimized local AI recipes submitted by the community. Run them instantly on your hardware.
        </p>
      </div>

      {/* Main Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-8 items-start w-full mt-8">
        
        {/* Left Column: Sidebar Filters */}
        <div className="lg:col-span-1 flex flex-col border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 p-6 rounded-none relative">
          {/* SVG Corner L-Brackets */}
          <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white pointer-events-none z-10">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] pointer-events-none z-10">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-y-[-1] pointer-events-none z-10">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none z-10">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          
          <div 
            onClick={() => setIsFiltersOpen(!isFiltersOpen)}
            className="flex items-center justify-between border-b border-zinc-200 dark:border-zinc-800 pb-3 cursor-pointer lg:cursor-default select-none"
          >
            <h3 className="font-mono text-xs font-bold text-zinc-900 dark:text-white uppercase tracking-wider flex items-center gap-2">
              Filters
              {(selectedVram.length > 0 || selectedPlatform.length > 0 || selectedVerification.length > 0) && (
                <span className="w-4 h-4 rounded-full bg-blue-600 text-white text-[9px] font-bold flex items-center justify-center">
                  {selectedVram.length + selectedPlatform.length + selectedVerification.length}
                </span>
              )}
            </h3>
            <div className="lg:hidden text-zinc-500 hover:text-black dark:hover:text-white">
              <svg 
                width="12" height="12" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg"
                className={`transform transition-transform duration-250 ${isFiltersOpen ? "rotate-180" : ""}`}
              >
                <path d="M4 6L7.5 9.5L11 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </div>
          </div>

          <div className={`lg:flex flex-col gap-8 mt-6 ${isFiltersOpen ? "flex" : "hidden"}`}>
            {/* VRAM Tiers */}
            <div className="flex flex-col gap-2.5">
              <h4 className="font-mono text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">VRAM Limits</h4>
              {["4GB", "8GB", "12GB", "24GB", "Unified"].map((vram) => (
                <label key={vram} className="flex items-center gap-2 cursor-pointer font-mono text-[10px] text-zinc-600 dark:text-zinc-300 hover:text-black dark:hover:text-white">
                  <input 
                    type="checkbox"
                    checked={selectedVram.includes(vram)}
                    onChange={() => toggleFilter(selectedVram, setSelectedVram, vram)}
                    className="rounded-none border-zinc-300 dark:border-zinc-800 text-blue-600 focus:ring-0 w-3 h-3 bg-transparent accent-blue-600"
                  />
                  {vram} {vram === "Unified" ? "Mac" : ""}
                </label>
              ))}
            </div>

            {/* Platform Tiers */}
            <div className="flex flex-col gap-2.5">
              <h4 className="font-mono text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">Platform Acceleration</h4>
              {["CUDA", "METAL", "ROCM", "CPU"].map((platform) => (
                <label key={platform} className="flex items-center gap-2 cursor-pointer font-mono text-[10px] text-zinc-600 dark:text-zinc-300 hover:text-black dark:hover:text-white">
                  <input 
                    type="checkbox"
                    checked={selectedPlatform.includes(platform)}
                    onChange={() => toggleFilter(selectedPlatform, setSelectedPlatform, platform)}
                    className="rounded-none border-zinc-300 dark:border-zinc-800 text-blue-600 focus:ring-0 w-3 h-3 bg-transparent accent-blue-600"
                  />
                  {platform}
                </label>
              ))}
            </div>

            {/* Verification Tiers */}
            <div className="flex flex-col gap-2.5">
              <h4 className="font-mono text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">Verification</h4>
              {[
                { label: "Verified Publisher", value: "publisher" },
                { label: "Community Verified", value: "community" }
              ].map((tier) => (
                <label key={tier.value} className="flex items-center gap-2 cursor-pointer font-mono text-[10px] text-zinc-600 dark:text-zinc-300 hover:text-black dark:hover:text-white">
                  <input 
                    type="checkbox"
                    checked={selectedVerification.includes(tier.value)}
                    onChange={() => toggleFilter(selectedVerification, setSelectedVerification, tier.value)}
                    className="rounded-none border-zinc-300 dark:border-zinc-800 text-blue-600 focus:ring-0 w-3 h-3 bg-transparent accent-blue-600"
                  />
                  {tier.label}
                </label>
              ))}
            </div>
          </div>

        </div>

        {/* Right Column: Catalog Grid, Search & Controls */}
        <div className="lg:col-span-3 flex flex-col gap-6 w-full">
          
          {/* Controls Bar */}
          <div className="flex flex-col sm:flex-row items-center gap-3 w-full">
            
            <div className="relative flex-grow h-9 w-full sm:w-auto">
              <div className="flex items-center h-full px-3 bg-black/5 dark:bg-white/5 border border-black/10 dark:border-white/10 rounded-none gap-2 transition-all duration-300 hover:border-black/20 dark:hover:border-white/20 focus-within:border-blue-500 group/search relative">
                {/* SVG Corner L-Brackets */}
                <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-1.5 h-1.5 fill-black dark:fill-white pointer-events-none z-10">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-1.5 h-1.5 fill-black dark:fill-white scale-x-[-1] pointer-events-none z-10">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-1.5 h-1.5 fill-black dark:fill-white scale-y-[-1] pointer-events-none z-10">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-1.5 h-1.5 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none z-10">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg width="12" height="12" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className="text-black/40 dark:text-white/40 group-hover/search:text-black/60 dark:group-hover/search:text-white/60">
                  <path d="M14.5 14.5L10.5 10.5M12.5 6.5C12.5 9.81371 9.81371 12.5 6.5 12.5C3.18629 12.5 0.5 9.81371 0.5 6.5C0.5 3.18629 3.18629 0.5 6.5 0.5C9.81371 0.5 12.5 3.18629 12.5 6.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search models, quantizations, creators..."
                  className="w-full bg-transparent border-none outline-none font-mono text-xs text-black dark:text-white placeholder-black/40 dark:placeholder-white/40 leading-none h-full"
                />
              </div>
            </div>

            <div className="flex items-center gap-2 w-full sm:w-auto shrink-0">
              <ShortcutButton
                text="Model"
                shortcutKey="1"
                variant={viewMode === "model" ? "black" : "white"}
                onClick={() => {
                  setViewMode(viewMode === "model" ? "default" : "model");
                  setSelectedModel(null);
                  setSelectedGpu(null);
                }}
                className="w-1/2 sm:w-auto h-9"
              />
              <ShortcutButton
                text="GPU"
                shortcutKey="2"
                variant={viewMode === "gpu" ? "black" : "white"}
                onClick={() => {
                  setViewMode(viewMode === "gpu" ? "default" : "gpu");
                  setSelectedModel(null);
                  setSelectedGpu(null);
                }}
                className="w-1/2 sm:w-auto h-9"
              />
            </div>

          </div>

          {/* Breadcrumb Path */}
          {(selectedModel || selectedGpu) && (
            <div className="flex items-center gap-2 font-mono text-[10px] text-zinc-400 dark:text-zinc-500 py-1">
              <button 
                onClick={() => {
                  setSelectedModel(null);
                  setSelectedGpu(null);
                }} 
                className="hover:text-black dark:hover:text-white cursor-pointer"
              >
                Registry
              </button>
              <span>/</span>
              {selectedModel && (
                <>
                  <span className="text-zinc-600 dark:text-zinc-400">Models</span>
                  <span>/</span>
                  <span className="text-blue-600 dark:text-blue-400 font-bold">{selectedModel}</span>
                </>
              )}
              {selectedGpu && (
                <>
                  <span className="text-zinc-600 dark:text-zinc-400">GPUs</span>
                  <span>/</span>
                  <span className="text-blue-600 dark:text-blue-400 font-bold">{selectedGpu}</span>
                </>
              )}
              <button 
                onClick={() => {
                  setSelectedModel(null);
                  setSelectedGpu(null);
                }}
                className="ml-auto text-[#ff453a] hover:underline cursor-pointer"
              >
                Back to overview
              </button>
            </div>
          )}

          {/* View Case 1: Model Group Cards View */}
          {viewMode === "model" && !selectedModel && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 w-full">
              {uniqueModels.map((model) => (
                <div 
                  key={model.name}
                  onClick={() => setSelectedModel(model.name)}
                  className="group relative flex flex-col justify-between border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-6 rounded-none transition-colors hover:bg-zinc-200/50 dark:hover:bg-zinc-900/50 cursor-pointer min-h-[140px]"
                >
                  <svg viewBox="0 0 12 12" className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  
                  <div>
                    <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400">
                      {model.brand}
                    </span>
                    <h3 className="text-xl font-semibold font-switzer tracking-tight text-black dark:text-white mt-3 mb-1">
                      {model.name}
                    </h3>
                  </div>

                  <div className="flex justify-between items-center mt-6 border-t border-zinc-200 dark:border-zinc-800/80 pt-3 font-mono text-[10px] text-zinc-400 dark:text-zinc-500">
                    <span>{model.count} {model.count === 1 ? "recipe" : "recipes"} available</span>
                    <span className="text-blue-600 dark:text-blue-400 group-hover:underline">Explore recipes →</span>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* View Case 2: GPU Group Cards View */}
          {viewMode === "gpu" && !selectedGpu && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 w-full">
              {uniqueGpus.map((gpu) => (
                <div 
                  key={gpu.name}
                  onClick={() => setSelectedGpu(gpu.name)}
                  className="group relative flex flex-col justify-between border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-6 rounded-none transition-colors hover:bg-zinc-200/50 dark:hover:bg-zinc-900/50 cursor-pointer min-h-[140px]"
                >
                  <svg viewBox="0 0 12 12" className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>

                  <div>
                    <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400">
                      Telemetry Node
                    </span>
                    <h3 className="text-lg font-semibold font-switzer tracking-tight text-black dark:text-white mt-3 mb-1">
                      {gpu.name}
                    </h3>
                  </div>

                  <div className="flex justify-between items-center mt-6 border-t border-zinc-200 dark:border-zinc-800/80 pt-3 font-mono text-[10px] text-zinc-400 dark:text-zinc-500">
                    <span>Avg Speed: <strong className="text-zinc-850 dark:text-zinc-200">{gpu.speed} t/s</strong> ({gpu.count} recipes)</span>
                    <span className="text-blue-600 dark:text-blue-400 group-hover:underline">Explore recipes →</span>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* View Case 3: Flat / Filtered Recipes */}
          {((viewMode === "default") || (viewMode === "model" && selectedModel) || (viewMode === "gpu" && selectedGpu)) && (
            <>
              {filteredRecipes.length === 0 ? (
                <div className="w-full border border-dashed border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/30 dark:bg-[#171616]/30 py-16 text-center rounded-none relative">
                  <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2 h-2 fill-zinc-400 dark:fill-zinc-600 pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2 h-2 fill-zinc-400 dark:fill-zinc-600 scale-x-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2 h-2 fill-zinc-400 dark:fill-zinc-600 scale-y-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2 h-2 fill-zinc-400 dark:fill-zinc-600 scale-x-[-1] scale-y-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <p className="font-mono text-xs text-zinc-400 dark:text-zinc-500">No matching model recipes found in this view.</p>
                </div>
              ) : (
                <div className="flex flex-col gap-6 w-full">
                  {filteredRecipes.map((recipe) => (
                    <div 
                      key={recipe.id}
                      onClick={() => router.push(`/recipes/${recipe.id}`)}
                      className="group relative flex flex-col justify-between border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-6 rounded-none transition-colors min-h-[220px] cursor-pointer hover:bg-zinc-200/30 dark:hover:bg-zinc-900/30"
                    >
                      <svg 
                        viewBox="0 0 12 12" 
                        className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none transition-transform duration-200 group-hover:-translate-x-1 group-hover:-translate-y-1"
                      >
                        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                      </svg>
                      <svg 
                        viewBox="0 0 12 12" 
                        className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-transform duration-200 group-hover:translate-x-1 group-hover:-translate-y-1"
                      >
                        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                      </svg>
                      <svg 
                        viewBox="0 0 12 12" 
                        className="absolute bottom-0 left-0 w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-transform duration-200 group-hover:-translate-x-1 group-hover:translate-y-1"
                      >
                        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                      </svg>
                      <svg 
                        viewBox="0 0 12 12" 
                        className="absolute bottom-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-transform duration-200 group-hover:translate-x-1 group-hover:translate-y-1"
                      >
                        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                      </svg>

                      {/* Header Row */}
                      <div className="flex items-start justify-between gap-4 border-b border-zinc-200 dark:border-zinc-800 pb-4">
                        <div>
                          <div className="flex items-center gap-2 mb-2">
                            <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-550 dark:text-zinc-400 bg-zinc-200/50 dark:bg-zinc-900/50">
                              {recipe.hardware.targetPlatform.toUpperCase()}
                            </span>
                            <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 bg-zinc-200/50 dark:bg-zinc-900/50 text-zinc-850 dark:text-zinc-200">
                              {recipe.hardware.minVram} VRAM
                            </span>
                            {recipe.verified === "publisher" && (
                              <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-blue-500/20 bg-blue-500/10 text-blue-600 dark:text-blue-400 font-bold">
                                Verified Org
                              </span>
                            )}
                            {recipe.verified === "community" && (
                              <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-bold">
                                Community Verified
                              </span>
                            )}
                          </div>
                          <h2 className="text-xl font-semibold tracking-tight font-switzer text-black dark:text-white">
                            <Link 
                              href={`/users/${recipe.creator}`} 
                              onClick={(e) => e.stopPropagation()} 
                              className="text-black dark:text-white hover:text-blue-600 dark:hover:text-blue-400 underline decoration-dotted transition-colors"
                            >
                              {recipe.creator}
                            </Link>
                            /<span className="text-blue-600 dark:text-blue-400">{recipe.name}</span>
                          </h2>
                        </div>

                        {user && (
                          <button
                            onClick={(e) => toggleStar(e, recipe.id)}
                            className={`p-2 border transition-all shrink-0 cursor-pointer ${
                              starredRecipeIds.includes(recipe.id)
                                ? "border-amber-400 bg-amber-400/10 text-amber-500"
                                : "border-zinc-300 dark:border-zinc-850 text-zinc-400 hover:text-black dark:hover:text-white hover:bg-zinc-200/40 dark:hover:bg-zinc-850/40"
                            }`}
                          >
                            <Star className={`w-3.5 h-3.5 ${starredRecipeIds.includes(recipe.id) ? "fill-amber-500" : ""}`} />
                          </button>
                        )}
                      </div>

                      {/* Description */}
                      <div className="mt-4">
                        <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm leading-relaxed mb-3">
                          {recipe.description}
                        </p>
                        <div className="font-mono text-xs text-zinc-400 dark:text-zinc-500">
                          <span className="font-bold text-zinc-500 dark:text-zinc-400">Base Model:</span> {recipe.baseModel}
                        </div>
                      </div>

                      {/* Clipboard copy row */}
                      <div className="flex items-center justify-between gap-4 mt-6">
                        <div className="w-full max-w-xl">
                          <button 
                            onClick={(e) => {
                              e.stopPropagation();
                              handleCopy(recipe.id);
                            }}
                            className="w-full flex items-center justify-between px-3 h-8 bg-zinc-200/50 dark:bg-zinc-900/50 hover:bg-zinc-200 dark:hover:bg-zinc-900 border border-zinc-300 dark:border-zinc-800 group/btn font-mono text-[10px] text-zinc-800 dark:text-zinc-200 cursor-pointer active:scale-[0.98] transition-all relative"
                          >
                            {/* Inner brackets for brutalist aesthetic */}
                            <svg viewBox="0 0 12 12" className="absolute top-0 left-0 w-1 h-1 fill-zinc-400 dark:fill-zinc-600 pointer-events-none">
                              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                            </svg>
                            <svg viewBox="0 0 12 12" className="absolute top-0 right-0 w-1 h-1 fill-zinc-400 dark:fill-zinc-600 scale-x-[-1] pointer-events-none">
                              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                            </svg>
                            <svg viewBox="0 0 12 12" className="absolute bottom-0 left-0 w-1 h-1 fill-zinc-400 dark:fill-zinc-600 scale-y-[-1] pointer-events-none">
                              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                            </svg>
                            <svg viewBox="0 0 12 12" className="absolute bottom-0 right-0 w-1 h-1 fill-zinc-400 dark:fill-zinc-600 scale-x-[-1] scale-y-[-1] pointer-events-none">
                              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                            </svg>
                            <span className="truncate select-text">bloc deploy {recipe.id}</span>
                            <span className="flex-shrink-0 ml-4 font-bold uppercase text-[9px] text-zinc-400 group-hover/btn:text-blue-600 dark:group-hover/btn:text-blue-400 transition-colors">
                              {copiedId === recipe.id ? "Copied!" : "Copy"}
                            </span>
                          </button>
                        </div>

                        <div className="flex items-center gap-1.5 font-mono text-[10px] text-zinc-500 dark:text-zinc-400 select-none shrink-0">
                          <Download className="w-3.5 h-3.5" />
                          <span>{recipe.telemetry.runs}</span>
                        </div>
                      </div>

                    </div>
                  ))}
                </div>
              )}
            </>
          )}

        </div>

      </div>

    </div>
  );
}
