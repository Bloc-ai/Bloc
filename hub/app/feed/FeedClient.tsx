"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { supabase } from "@/lib/supabase";
import { Recipe } from "@/lib/registry-data";
import { Download, Star, Loader2, KeyRound, Compass, ArrowRight, CornerDownRight } from "lucide-react";
import { toast } from "sonner";

export default function FeedClient() {
  const { user, loading } = useAuth();
  const router = useRouter();

  // Followed Usernames State
  const [followedUsernames, setFollowedUsernames] = useState<string[]>([]);
  const [loadingFeed, setLoadingFeed] = useState(true);

  // Supabase recipes from followed users
  const [dbFeedRecipes, setDbFeedRecipes] = useState<Recipe[]>([]);

  // Starred Recipes state
  const [starredRecipeIds, setStarredRecipeIds] = useState<string[]>([]);

  // Rate limiting / spam protection ref for stars
  const pendingStarsRef = useRef<Set<string>>(new Set());

  // Telemetry copy command clipboard notifier
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const copyTimeoutRef = useRef<any>(null);

  // Redirect unauthenticated users
  useEffect(() => {
    if (!loading && !user) {
      router.replace("/login?next=/feed");
    }
  }, [user, loading, router]);

  useEffect(() => {
    return () => {
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    };
  }, []);

  // Sync followed users from Supabase if logged in
  useEffect(() => {
    let active = true;
    async function fetchFollowedUsers() {
      if (!supabase || !user) {
        setLoadingFeed(false);
        return;
      }

      try {
        const { data, error } = await supabase
          .from("follows")
          .select(`
            following_id,
            profiles:following_id (username)
          `)
          .eq("follower_id", user.id);

        if (error) throw error;

        if (active) {
          const usernames = (data || [])
            .map((f: any) => f.profiles?.username)
            .filter(Boolean);
          setFollowedUsernames(usernames);
        }
      } catch (err: any) {
        console.error("Error fetching followed users for feed:", err);
      } finally {
        if (active) {
          setLoadingFeed(false);
        }
      }
    }

    fetchFollowedUsers();
    return () => {
      active = false;
    };
  }, [user]);

  // Fetch Supabase recipes from followed creators
  useEffect(() => {
    let active = true;
    async function fetchFeedRecipes() {
      if (!supabase || !user || followedUsernames.length === 0) {
        if (active) setDbFeedRecipes([]);
        return;
      }
      try {
        const { data, error } = await supabase
          .from("recipes")
          .select(
            "id, name, creator, description, base_model, min_vram, target_platform, yaml_content, compat_builds, created_at, telemetry_events(count)"
          )
          .in("creator", followedUsernames)
          .order("created_at", { ascending: false })
          .limit(100);
        if (error) throw error;
        if (data && active) {
          const mapped: Recipe[] = data.map((row: any) => {
            const quantMatch = row.yaml_content?.match(
              /quantization:\s*(?:"([^"]+)"|'([^"]+)'|([a-zA-Z0-9_-]+))/
            );
            const quantization = quantMatch
              ? quantMatch[1] || quantMatch[2] || quantMatch[3]
              : "Q4_K_M";
            return {
              id: `${row.creator}/${row.name}`,
              name: row.name,
              creator: row.creator,
              description: row.description || "",
              baseModel: row.base_model,
              engine: "llama.cpp",
              quantization,
              hardware: {
                minVram: row.min_vram,
                targetPlatform: row.target_platform as any,
              },
              verified: "none" as const,
              telemetry: {
                runs: row.telemetry_events?.[0]?.count || 0,
                benchmarks: [],
              },
            };
          });
          setDbFeedRecipes(mapped);
        }
      } catch (err) {
        console.error("Error fetching feed recipes:", err);
      }
    }
    fetchFeedRecipes();
    return () => { active = false; };
  }, [user, followedUsernames]);

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
        console.error("Error loading stars for feed:", err);
      }
    }
    loadStars();
    return () => {
      active = false;
    };
  }, [user]);

  // Copy command to clipboard
  const handleCopy = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    const command = `bloc deploy ${id}`;
    navigator.clipboard.writeText(command);
    setCopiedId(id);
    if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    copyTimeoutRef.current = setTimeout(() => setCopiedId(null), 1500);
  };

  // Toggle Star action (database backed)
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

  // Loading global state
  if (loading) {
    return (
      <div className="flex-grow flex items-center justify-center min-h-[80vh]">
        <Loader2 className="w-6 h-6 animate-spin text-zinc-500" />
      </div>
    );
  }

  // unauthenticated view (Login Gate)
  if (!user) {
    return (
      <div className="flex-grow flex flex-col items-center justify-center p-6 min-h-[85vh] relative">
        {/* Grid Background */}
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#8080800a_1px,transparent_1px),linear-gradient(to_bottom,#8080800a_1px,transparent_1px)] bg-[size:14px_24px] pointer-events-none -z-10" />

        <div className="relative w-full max-w-lg border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-8 md:p-10 rounded-none shadow-xl text-center">
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

          <div className="flex items-center justify-center w-12 h-12 bg-blue-500/10 border border-blue-500/20 text-blue-600 dark:text-blue-400 mx-auto mb-6">
            <KeyRound className="w-5 h-5" />
          </div>

          <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-blue-500/20 bg-blue-500/5 text-blue-600 dark:text-blue-400 font-bold">
            Authentication Required
          </span>

          <h1 className="text-2xl font-semibold tracking-tight font-switzer text-black dark:text-white mt-4 leading-tight">
            Developer Feed is Locked
          </h1>
          <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-xs leading-relaxed mt-2.5 max-w-sm mx-auto">
            Sign in to your GitHub account to dynamically stream model configurations, parameters, and local setup recipes from engineers you follow.
          </p>

          <div className="mt-8 pt-6 border-t border-zinc-200 dark:border-zinc-800">
            <button
              onClick={() => router.push("/login")}
              className="group relative w-full h-11 border border-zinc-850 dark:border-zinc-100 bg-black text-white dark:bg-white dark:text-black hover:bg-zinc-800 dark:hover:bg-zinc-100 font-mono text-[11px] uppercase font-bold tracking-wider cursor-pointer transition-all rounded-none flex items-center justify-center gap-3"
            >
              Sign In with GitHub
              <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-0.5" />
            </button>
          </div>
        </div>
      </div>
    );
  }

  const feedRecipes = dbFeedRecipes;

  return (
    <div className="max-w-4xl w-full mx-auto px-6 py-16 pt-24 min-h-screen relative">
      {/* Grid Background */}
      <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808005_1px,transparent_1px),linear-gradient(to_bottom,#80808005_1px,transparent_1px)] bg-[size:14px_24px] pointer-events-none -z-10" />

      {/* Feed Header */}
      <div className="mb-10 text-left border-b border-zinc-200 dark:border-zinc-800 pb-8">
        <h1 className="text-4xl font-semibold tracking-tight font-switzer text-black dark:text-white leading-none">
          Following [{followedUsernames.length}]
        </h1>
      </div>

      {loadingFeed ? (
        <div className="py-20 flex items-center justify-center">
          <Loader2 className="w-6 h-6 animate-spin text-zinc-500" />
        </div>
      ) : feedRecipes.length === 0 ? (
        /* Empty State */
        <div className="relative w-full border border-dashed border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/30 dark:bg-[#171616]/30 p-10 md:p-14 text-center rounded-none shadow-sm">
          <div className="flex items-center justify-center w-12 h-12 bg-zinc-200/50 dark:bg-zinc-900/50 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400 mx-auto mb-6">
            <Compass className="w-5 h-5" />
          </div>
          
          <h3 className="font-switzer font-bold text-lg text-black dark:text-white leading-tight">
            Your Stream is Silent
          </h3>
          <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-xs leading-relaxed mt-2 max-w-md mx-auto">
            {followedUsernames.length === 0 
              ? "You aren't following anyone yet. Head over to the model registry to connect with local hardware AI optimization engineers."
              : "The engineers you follow haven't published any optimization recipes yet. Check back soon for active parameter uploads!"}
          </p>

          <button
            onClick={() => router.push("/registry")}
            className="group mt-8 inline-flex items-center justify-center gap-2.5 px-6 h-10 border border-zinc-850 dark:border-zinc-100 bg-black text-white dark:bg-white dark:text-black hover:bg-zinc-800 dark:hover:bg-zinc-100 font-mono text-[10px] uppercase font-bold tracking-wider cursor-pointer transition-all"
          >
            Explore Registry
            <ArrowRight className="w-3.5 h-3.5 transition-transform group-hover:translate-x-0.5" />
          </button>
        </div>
      ) : (
        /* Feed Recipe Stream */
        <div className="flex flex-col gap-6">
          {feedRecipes.map((recipe) => {
            const isStarred = starredRecipeIds.includes(recipe.id);
            return (
              <div 
                key={recipe.id}
                onClick={() => router.push(`/recipes/${recipe.id}`)}
                className="group relative flex flex-col justify-between border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-6 rounded-none transition-colors min-h-[220px] cursor-pointer hover:bg-zinc-200/30 dark:hover:bg-zinc-900/30"
              >
                {/* SVG Corner L-Brackets */}
                <svg viewBox="0 0 12 12" className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none transition-transform duration-200 group-hover:-translate-x-1 group-hover:-translate-y-1">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-transform duration-200 group-hover:translate-x-1 group-hover:-translate-y-1">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute bottom-0 left-0 w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-transform duration-200 group-hover:-translate-x-1 group-hover:translate-y-1">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute bottom-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-transform duration-200 group-hover:translate-x-1 group-hover:translate-y-1">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>

                {/* Header Info */}
                <div className="flex items-start justify-between gap-4 border-b border-zinc-200 dark:border-zinc-800 pb-4">
                  <div>
                    <div className="flex items-center gap-2 mb-2">
                      <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400">
                        {recipe.hardware.targetPlatform.toUpperCase()}
                      </span>
                      <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 bg-zinc-200 dark:bg-zinc-900 text-zinc-850 dark:text-zinc-200">
                        {recipe.hardware.minVram} VRAM
                      </span>
                      {recipe.verified !== "none" && (
                        <span className="font-mono text-[8px] uppercase tracking-wider text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-1.5 py-0.5 font-bold">
                          {recipe.verified} verified
                        </span>
                      )}
                    </div>
                    <h2 className="text-xl font-semibold tracking-tight font-switzer text-black dark:text-white">
                      <Link href={`/users/${recipe.creator}`} className="hover:text-blue-500 transition-colors" onClick={(e) => e.stopPropagation()}>
                        {recipe.creator}
                      </Link>
                      /<span className="text-blue-600 dark:text-blue-400">{recipe.name}</span>
                    </h2>
                  </div>

                  <button
                    onClick={(e) => toggleStar(e, recipe.id)}
                    className={`p-2 border transition-all ${
                      isStarred
                        ? "border-amber-400 bg-amber-400/10 text-amber-500"
                        : "border-zinc-300 dark:border-zinc-850 text-zinc-400 hover:text-black dark:hover:text-white hover:bg-zinc-200/40 dark:hover:bg-zinc-850/40"
                    }`}
                  >
                    <Star className={`w-3.5 h-3.5 ${isStarred ? "fill-amber-500" : ""}`} />
                  </button>
                </div>

                {/* Description & Base Model */}
                <div className="mt-4">
                  <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm leading-relaxed mb-3">
                    {recipe.description}
                  </p>
                  <div className="font-mono text-xs text-zinc-400 dark:text-zinc-500 flex items-center gap-1.5">
                    <CornerDownRight className="w-3.5 h-3.5" />
                    <span className="font-bold text-zinc-500 dark:text-zinc-400">Base Model:</span> {recipe.baseModel}
                  </div>
                </div>

                {/* Deploy Action Copy Command Row */}
                <div className="flex items-center justify-between gap-4 mt-6">
                  <div className="w-full max-w-xl">
                    <button 
                      onClick={(e) => handleCopy(e, recipe.id)}
                      className="w-full flex items-center justify-between px-3 h-8 bg-zinc-200/50 dark:bg-zinc-900/50 hover:bg-zinc-200 dark:hover:bg-zinc-900 border border-zinc-300 dark:border-zinc-800 group/btn font-mono text-[10px] text-zinc-850 dark:text-zinc-200 cursor-pointer active:scale-[0.98] transition-all"
                    >
                      <span className="truncate select-text">bloc deploy {recipe.id}</span>
                      <span className="flex-shrink-0 ml-4 font-bold uppercase text-[9px] text-zinc-400 group-hover/btn:text-blue-600 dark:group-hover/btn:text-blue-400 transition-colors">
                        {copiedId === recipe.id ? "Copied!" : "Copy"}
                      </span>
                    </button>
                  </div>

                  <div className="flex items-center gap-1.5 font-mono text-[10px] text-zinc-500 dark:text-zinc-400 select-none shrink-0">
                    <Download className="w-3.5 h-3.5" />
                    <span>{recipe.telemetry.runs} runs</span>
                  </div>
                </div>

              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
