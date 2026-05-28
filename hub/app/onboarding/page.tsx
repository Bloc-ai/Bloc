"use client";

import React, { useState, useEffect } from "react";
import { useAuth } from "@/context/AuthContext";
import { useRouter } from "next/navigation";
import { supabase } from "@/lib/supabase";
import { toast } from "sonner";
import { Loader2, ArrowRight } from "lucide-react";

export default function OnboardingPage() {
  const { user, loading } = useAuth();
  const router = useRouter();

  const [displayName, setDisplayName] = useState("");
  const [username, setUsername] = useState("");
  const [location, setLocation] = useState("");
  const [bio, setBio] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Sync initials from user session metadata
  useEffect(() => {
    if (user) {
      // Default username to visitor handle or blank
      setUsername(user.username || "");
    }
  }, [user]);

  // Protect route
  useEffect(() => {
    if (!loading && !user) {
      router.push("/login");
    }
  }, [user, loading, router]);

  if (loading || !user) {
    return (
      <div className="flex-grow flex items-center justify-center min-h-[70vh]">
        <Loader2 className="w-6 h-6 animate-spin text-zinc-500" />
      </div>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim()) {
      toast.error("Please enter a valid unique username");
      return;
    }

    setIsSubmitting(true);
    const cleanUsername = username.toLowerCase().trim().replace(/\s+/g, "");

    if (supabase) {
      try {
        // 1. Verify username is not claimed by another developer
        const { data: existing } = await supabase
          .from("profiles")
          .select("username")
          .eq("username", cleanUsername)
          .maybeSingle();

        if (existing) {
          toast.error(`Username @${cleanUsername} is already claimed!`);
          setIsSubmitting(false);
          return;
        }

        // 2. Insert new profile details
        const { error } = await supabase.from("profiles").insert({
          auth_id: user.id,
          username: cleanUsername,
          display_name: displayName.trim() || user.username || cleanUsername,
          location: location.trim(),
          bio: bio.trim(),
          avatar_url: user.avatar_url,
        });

        if (error) throw error;

        toast.success("Profile claimed successfully!");
        
        // Brief timeout for session context synchronization
        setTimeout(() => {
          window.location.href = "/registry";
        }, 1000);
      } catch (err: any) {
        toast.error("Error creating profile", {
          description: err.message || "Please check your network and try again.",
        });
        setIsSubmitting(false);
      }
    } else {
      // Local Mock Dev loop
      toast.success("Mock profile claimed successfully!");
      setTimeout(() => {
        router.push("/registry");
      }, 1000);
    }
  };

  return (
    <div className="flex-grow flex flex-col items-center justify-center p-6 min-h-[85vh] relative select-none">
      
      <div className="relative w-full max-w-xl border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-8 md:p-10 rounded-none shadow-xl">
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

        <div className="text-center mb-8">
          <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-blue-500/20 bg-blue-500/5 text-blue-600 dark:text-blue-400 font-bold">
            Profile Claim
          </span>
          <h1 className="text-3xl font-semibold tracking-tight font-switzer text-black dark:text-white mt-4 leading-tight">
            Claim Your Developer Handle
          </h1>
          <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-xs leading-relaxed mt-2.5 max-w-md mx-auto">
            You are logged in! Complete your profile to claim your namespace for publishing model configurations and recipes.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">
              Verified GitHub Handle (Read-Only)
            </label>
            <div className="relative w-full h-10 border border-zinc-300 dark:border-zinc-800 bg-zinc-200/40 dark:bg-zinc-900/40 px-3 flex items-center justify-between font-mono text-sm text-zinc-550 dark:text-zinc-400 select-none">
              <span>@{username || "retrieving..."}</span>
              <div className="flex items-center gap-1.5 font-mono text-[8px] uppercase tracking-wider text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 font-bold">
                Verified OAuth
              </div>
            </div>
          </div>

          <div>
            <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">
              Display Name
            </label>
            <input
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="e.g. Jane Smith"
              className="w-full h-10 px-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-sm font-switzer outline-none focus:border-blue-500 rounded-none"
            />
          </div>

          <div>
            <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">
              Location
            </label>
            <input
              type="text"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              placeholder="e.g. San Francisco, CA"
              className="w-full h-10 px-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-sm font-switzer outline-none focus:border-blue-500 rounded-none"
            />
          </div>

          <div>
            <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">
              Developer Bio
            </label>
            <textarea
              rows={3}
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              placeholder="Tell the community about your local hardware quantizations and optimization setups..."
              className="w-full p-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-sm font-switzer outline-none focus:border-blue-500 rounded-none resize-none"
            />
          </div>

          <div className="pt-4 border-t border-zinc-200 dark:border-zinc-800">
            <button
              type="submit"
              disabled={isSubmitting}
              className="group relative w-full h-11 border border-zinc-850 dark:border-zinc-100 bg-black text-white dark:bg-white dark:text-black hover:bg-zinc-800 dark:hover:bg-zinc-100 font-mono text-[11px] uppercase font-bold tracking-wider cursor-pointer transition-all rounded-none flex items-center justify-center gap-3 disabled:opacity-75 disabled:cursor-not-allowed"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Creating Profile...
                </>
              ) : (
                <>
                  Claim and Enter Registry
                  <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-0.5" />
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
