"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import { registryRecipes, Recipe } from "@/lib/registry-data";
import { Download, Users, Star, User, Edit3, Check, X, MapPin, ArrowRight } from "lucide-react";
import { useAuth } from "@/context/AuthContext";
import { supabase } from "@/lib/supabase";
import { toast } from "sonner";

function GithubIcon({ className = "" }: { className?: string }) {
  return (
    <svg 
      className={className} 
      viewBox="0 0 24 24" 
      fill="none" 
      stroke="currentColor" 
      strokeWidth="2" 
      strokeLinecap="round" 
      strokeLinejoin="round"
    >
      <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
      <path d="M9 18c-4.51 2-5-2-7-2" />
    </svg>
  );
}

function TwitterIcon({ className = "" }: { className?: string }) {
  return (
    <svg 
      className={className} 
      viewBox="0 0 24 24" 
      fill="none" 
      stroke="currentColor" 
      strokeWidth="2" 
      strokeLinecap="round" 
      strokeLinejoin="round"
    >
      <path d="M22 4s-.7 2.1-2 3.4c1.6 10-9.4 17.3-18 11.6 2.2.1 4.4-.6 6-2C3 15.5.5 9.6 3 5c2.2 2.6 5.6 4.1 9 4-.9-4.2 4-6.6 7-3.8 1.1 0 3-1.2 3-1.2z" />
    </svg>
  );
}

function LinkedinIcon({ className = "" }: { className?: string }) {
  return (
    <svg 
      className={className} 
      viewBox="0 0 24 24" 
      fill="none" 
      stroke="currentColor" 
      strokeWidth="2" 
      strokeLinecap="round" 
      strokeLinejoin="round"
    >
      <path d="M16 8a6 6 0 0 1 6 6v7h-4v-7a2 2 0 0 0-2-2 2 2 0 0 0-2 2v7h-4v-7a6 6 0 0 1 6-6z" />
      <rect x="2" y="9" width="4" height="12" />
      <circle cx="4" cy="4" r="2" />
    </svg>
  );
}

// No mock developer profiles fallback - all profiles are loaded from database dynamically.

export default function UserProfilePage() {
  const params = useParams();
  const router = useRouter();
  const rawUsername = (params.username as string) || "";
  const usernameKey = rawUsername.toLowerCase();
  
  const { user } = useAuth();
  const isSelf = user?.username?.toLowerCase() === usernameKey;

  // Profiles State
  const [profiles, setProfiles] = useState<Record<string, any>>({});

  // Load profile from Supabase dynamically if available
  useEffect(() => {
    let active = true;
    async function loadDbProfile() {
      if (!supabase) return;
      try {
        const { data, error } = await supabase
          .from("profiles")
          .select("*")
          .eq("username", usernameKey)
          .maybeSingle();

        if (error) throw error;
        if (data && active) {
          // Fetch real counts from follows table
          const [followersCountRes, followingCountRes, isFollowingRes] = await Promise.all([
            supabase.from("follows").select("follower_id", { count: "exact", head: true }).eq("following_id", data.auth_id),
            supabase.from("follows").select("following_id", { count: "exact", head: true }).eq("follower_id", data.auth_id),
            user ? supabase.from("follows").select("*").eq("follower_id", user.id).eq("following_id", data.auth_id).maybeSingle() : Promise.resolve({ data: null })
          ]);

          setProfiles(prev => ({
            ...prev,
            [usernameKey]: {
              displayName: data.display_name || rawUsername,
              bio: data.bio || "Local AI developer and registry contributor.",
              location: data.location || "Unknown Location",
              github: data.username,
              twitter: data.twitter || "",
              linkedin: data.linkedin || "",
              avatarUrl: `https://github.com/${data.username}.png`,
              role: data.role || "Contributor",
              followersCount: followersCountRes.count || 0,
              followingCount: followingCountRes.count || 0,
              authId: data.auth_id
            }
          }));

          if (isFollowingRes.data) {
            setFollowedUsers(prev => ({
              ...prev,
              [usernameKey]: true
            }));
          } else {
            setFollowedUsers(prev => ({
              ...prev,
              [usernameKey]: false
            }));
          }
        }
      } catch (err) {
        console.error("Error loading profile from Supabase:", err);
      }
    }
    loadDbProfile();
    return () => {
      active = false;
    };
  }, [usernameKey, user]);
  const profile = profiles[usernameKey] || {
    displayName: rawUsername,
    bio: "Local AI developer and registry contributor.",
    location: "Unknown Location",
    github: rawUsername,
    twitter: "",
    linkedin: "",
    avatarUrl: `https://github.com/${usernameKey}.png`,
    role: "Contributor",
    followersCount: 0,
    followingCount: 0
  };

  // Follow State
  const [followedUsers, setFollowedUsers] = useState<Record<string, boolean>>({});
  const isFollowingThisUser = followedUsers[usernameKey] || false;

  // Active Tab
  const [activeTab, setActiveTab] = useState<"recipes" | "starred">("recipes");

  // Modals States
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isFollowersModalOpen, setIsFollowersModalOpen] = useState(false);
  const [isFollowingModalOpen, setIsFollowingModalOpen] = useState(false);

  // M1: Rate limiting refs
  const pendingStarsRef = useRef<Set<string>>(new Set());
  const pendingFollowRef = useRef<boolean>(false);

  // Supabase recipes state
  const [dbRecipes, setDbRecipes] = useState<Recipe[]>([]);

  // Load recipes dynamically from Supabase
  useEffect(() => {
    let active = true;
    async function loadDbRecipes() {
      if (!supabase) return;
      try {
        const { data, error } = await supabase
          .from("recipes")
          .select("id, name, creator, description, base_model, min_vram, target_platform, yaml_content, compat_builds, created_at, telemetry_events(count)")
          .eq("creator", usernameKey)
          .order("created_at", { ascending: false })
          .limit(100);
        
        if (error) throw error;
        
        if (data && active) {
          const mapped = data.map((row: any) => {
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
          setDbRecipes(mapped);
        }
      } catch (e) {
        console.error("Error loading recipes from Supabase:", e);
      }
    }
    loadDbRecipes();
    return () => {
      active = false;
    };
  }, [usernameKey]);

  // Dynamic followers/following list for owner management
  const [followersList, setFollowersList] = useState<string[]>([]);
  const [followingList, setFollowingList] = useState<string[]>([]);

  // Load lists of followers and following dynamically from Supabase
  useEffect(() => {
    let active = true;
    async function loadFollowLists() {
      if (!supabase) return;
      
      try {
        const { data: targetUser } = await supabase
          .from("profiles")
          .select("auth_id")
          .eq("username", usernameKey)
          .maybeSingle();
          
        if (!targetUser) return;
        
        // Fetch followers with full profile details
        const { data: followersData } = await supabase
          .from("follows")
          .select(`
            follower_id,
            profiles:follower_id (auth_id, username, display_name, avatar_url)
          `)
          .eq("following_id", targetUser.auth_id);
          
        // Fetch following with full profile details
        const { data: followingData } = await supabase
          .from("follows")
          .select(`
            following_id,
            profiles:following_id (auth_id, username, display_name, avatar_url)
          `)
          .eq("follower_id", targetUser.auth_id);

        if (active) {
          const followers: string[] = [];
          const following: string[] = [];
          const loadedProfiles: Record<string, any> = {};

          if (followersData) {
            followersData.forEach((f: any) => {
              const p = f.profiles;
              if (p && p.username) {
                followers.push(p.username);
                loadedProfiles[p.username.toLowerCase()] = {
                  displayName: p.display_name || p.username,
                  role: p.role || "Contributor",
                  avatarUrl: p.avatar_url || "",
                  github: p.username,
                  authId: p.auth_id
                };
              }
            });
          }

          if (followingData) {
            followingData.forEach((f: any) => {
              const p = f.profiles;
              if (p && p.username) {
                following.push(p.username);
                loadedProfiles[p.username.toLowerCase()] = {
                  displayName: p.display_name || p.username,
                  role: p.role || "Contributor",
                  avatarUrl: p.avatar_url || "",
                  github: p.username,
                  authId: p.auth_id
                };
              }
            });
          }

          setFollowersList(followers);
          setFollowingList(following);
          setProfiles(prev => ({
            ...prev,
            ...loadedProfiles
          }));
        }
      } catch (e) {
        console.error("Error loading follow lists:", e);
      }
    }
    loadFollowLists();
    return () => {
      active = false;
    };
  }, [usernameKey, user]);

  // Edit Profile Form State
  const [editForm, setEditForm] = useState({
    displayName: profile.displayName,
    bio: profile.bio,
    location: profile.location,
    github: profile.github,
    twitter: profile.twitter || "",
    linkedin: profile.linkedin || "",
  });

  // Keep form in sync when profile updates or is loaded
  useEffect(() => {
    setEditForm({
      displayName: profile.displayName,
      bio: profile.bio,
      location: profile.location,
      github: profile.github,
      twitter: profile.twitter || "",
      linkedin: profile.linkedin || "",
    });
  }, [usernameKey, profiles]);

  // Starred Recipes state
  const [starredRecipeIds, setStarredRecipeIds] = useState<string[]>([]);
  const [profileStarredIds, setProfileStarredIds] = useState<string[]>([]);

  // Load stars dynamically from Supabase
  useEffect(() => {
    let active = true;
    async function loadStars() {
      if (!supabase) return;
      try {
        const { data: targetUser } = await supabase
          .from("profiles")
          .select("auth_id")
          .eq("username", usernameKey)
          .maybeSingle();

        if (!targetUser) return;

        // Fetch visited profile's starred configurations
        const { data: profileStars, error: profileStarsError } = await supabase
          .from("stars")
          .select("recipe_id")
          .eq("user_id", targetUser.auth_id);

        if (profileStarsError) throw profileStarsError;

        if (active && profileStars) {
          const profileStarred = profileStars.map((s: any) => s.recipe_id);
          setProfileStarredIds(profileStarred);

          if (user && user.id === targetUser.auth_id) {
            // Self: viewer stars are the same as profile stars
            setStarredRecipeIds(profileStarred);
          } else if (user) {
            // Visitor: fetch active viewer's stars
            const { data: viewerStars, error: viewerStarsError } = await supabase
              .from("stars")
              .select("recipe_id")
              .eq("user_id", user.id);
            
            if (viewerStarsError) throw viewerStarsError;
            if (active && viewerStars) {
              setStarredRecipeIds(viewerStars.map((s: any) => s.recipe_id));
            }
          } else {
            // Unauthenticated: viewer stars is empty
            setStarredRecipeIds([]);
          }
        }
      } catch (e) {
        console.error("Error loading stars:", e);
      }
    }
    loadStars();
    return () => {
      active = false;
    };
  }, [usernameKey, user]);

  // Telemetry copy command clipboard notifier
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const copyTimeoutRef = useRef<any>(null);

  useEffect(() => {
    return () => {
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    };
  }, []);

  const handleCopy = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    const command = `bloc deploy ${id}`;
    navigator.clipboard.writeText(command);
    setCopiedId(id);
    if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    copyTimeoutRef.current = setTimeout(() => setCopiedId(null), 1500);
  };

  // Star / Unstar action (database backed)
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

    // Optimistic UI updates
    setStarredRecipeIds(prev =>
      nextState ? [...prev, id] : prev.filter(item => item !== id)
    );
    if (isSelf) {
      setProfileStarredIds(prev =>
        nextState ? [...prev, id] : prev.filter(item => item !== id)
      );
    }

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
      if (isSelf) {
        setProfileStarredIds(prev =>
          isCurrentlyStarred ? [...prev, id] : prev.filter(item => item !== id)
        );
      }
    } finally {
      pendingStarsRef.current.delete(id);
    }
  };

  // Follow/Unfollow action (database backed)
  const handleFollowToggle = async () => {
    if (!supabase || !user) {
      toast.error("Please sign in to follow this developer.");
      return;
    }

    if (pendingFollowRef.current) return;
    pendingFollowRef.current = true;

    const targetProf = profiles[usernameKey];
    const targetAuthId = targetProf?.authId;
    if (!targetAuthId) {
      toast.error("Developer profile is not fully loaded yet.");
      pendingFollowRef.current = false;
      return;
    }

    const nextState = !isFollowingThisUser;
    
    // Optimistic UI updates
    setFollowedUsers(prev => ({
      ...prev,
      [usernameKey]: nextState
    }));
    
    setProfiles(prev => {
      const currentProf = prev[usernameKey];
      if (!currentProf) return prev;
      return {
        ...prev,
        [usernameKey]: {
          ...currentProf,
          followersCount: nextState ? currentProf.followersCount + 1 : Math.max(0, currentProf.followersCount - 1)
        }
      };
    });

    try {
      if (nextState) {
        // Follow target
        const { error } = await supabase
          .from("follows")
          .insert({
            follower_id: user.id,
            following_id: targetAuthId
          });
        if (error) throw error;
        toast.success(`Successfully followed @${usernameKey}`);
      } else {
        // Unfollow target
        const { error } = await supabase
          .from("follows")
          .delete()
          .eq("follower_id", user.id)
          .eq("following_id", targetAuthId);
        if (error) throw error;
        toast.info(`Unfollowed @${usernameKey}`);
      }
    } catch (err: any) {
      // Revert optimistic updates on error
      setFollowedUsers(prev => ({
        ...prev,
        [usernameKey]: !nextState
      }));
      setProfiles(prev => {
        const currentProf = prev[usernameKey];
        if (!currentProf) return prev;
        return {
          ...prev,
          [usernameKey]: {
            ...currentProf,
            followersCount: !nextState ? currentProf.followersCount + 1 : Math.max(0, currentProf.followersCount - 1)
          }
        };
      });
      toast.error("Follow action failed", {
        description: err.message || "Please check your database connectivity."
      });
    } finally {
      pendingFollowRef.current = false;
    }
  };

  // Save profile changes
  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault();

    if (supabase && user) {
      try {
        const { error } = await supabase
          .from("profiles")
          .update({
            display_name: editForm.displayName,
            bio: editForm.bio,
            location: editForm.location,
            twitter: editForm.twitter,
            linkedin: editForm.linkedin,
          })
          .eq("auth_id", user.id);

        if (error) throw error;
        toast.success("Profile saved successfully!");
      } catch (err: any) {
        toast.error("Error saving profile details", {
          description: err.message || "Please check your network and try again."
        });
        return;
      }
    }


    setProfiles(prev => {
      const currentProf = prev[usernameKey];
      return {
        ...prev,
        [usernameKey]: {
          ...currentProf,
          displayName: editForm.displayName,
          bio: editForm.bio,
          location: editForm.location,
          twitter: editForm.twitter,
          linkedin: editForm.linkedin,
        }
      };
    });
    setIsEditModalOpen(false);
  };

  // Close modals on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setIsEditModalOpen(false);
        setIsFollowersModalOpen(false);
        setIsFollowingModalOpen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Filter recipes owned by this profile (Mock recipes + Supabase recipes)
  const userRecipes = [
    ...registryRecipes.filter((r) => r.creator.toLowerCase() === usernameKey),
    ...dbRecipes.filter((dbR) => dbR.creator.toLowerCase() === usernameKey && !registryRecipes.some((mockR) => mockR.id === dbR.id))
  ];

  // Starred recipes list (Mock recipes + Supabase recipes)
  const userStarredRecipes = [
    ...registryRecipes.filter((r) => profileStarredIds.includes(r.id)),
    ...dbRecipes.filter((dbR) => profileStarredIds.includes(dbR.id) && !registryRecipes.some((mockR) => mockR.id === dbR.id))
  ];



  return (
    <div className="max-w-4xl w-full mx-auto px-6 py-16 pt-24 min-h-screen">
      

      {/* Back to Registry */}
      <Link href="/registry" className="inline-flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors mb-8 group">
        <svg width="8" height="8" viewBox="0 0 10 10" fill="none" xmlns="http://www.w3.org/2000/svg" className="transform rotate-180 transition-transform group-hover:-translate-x-0.5">
          <path d="M1 9L9 1M9 1H1M9 1V9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
        Back to registry
      </Link>

      {/* Header Profile Card */}
      <div className="relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-8 rounded-none mb-8">
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

        <div className="flex flex-col md:flex-row md:items-start justify-between gap-6">
          <div className="flex flex-col sm:flex-row items-start gap-6 flex-1 w-full">
            {/* Profile Avatar (PFP) */}
            <div className="w-20 h-20 md:w-24 md:h-24 shrink-0 border border-zinc-300 dark:border-zinc-800 bg-zinc-200/50 dark:bg-zinc-900/50 flex items-center justify-center font-mono font-bold text-xl md:text-2xl text-zinc-800 dark:text-zinc-200 relative select-none">
              {/* Corner brackets */}
              <svg viewBox="0 0 12 12" className="absolute top-0 left-0 w-2 h-2 fill-zinc-400 dark:fill-zinc-600 pointer-events-none">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute top-0 right-0 w-2 h-2 fill-zinc-400 dark:fill-zinc-600 scale-x-[-1] pointer-events-none">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute bottom-0 left-0 w-2 h-2 fill-zinc-400 dark:fill-zinc-600 scale-y-[-1] pointer-events-none">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute bottom-0 right-0 w-2 h-2 fill-zinc-400 dark:fill-zinc-600 scale-x-[-1] scale-y-[-1] pointer-events-none">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              
              {profile.avatarUrl ? (
                <Image src={profile.avatarUrl} alt={profile.displayName} width={96} height={96} className="w-full h-full object-cover rounded-none" unoptimized />
              ) : (
                profile.displayName.split(" ").map((n: string) => n[0]).join("").substring(0, 2).toUpperCase()
              )}
            </div>

            {/* Main Info */}
            <div className="flex-1 w-full">
              <h1 className="text-3xl md:text-4xl font-semibold tracking-tight font-switzer text-black dark:text-white mb-2 leading-tight">
                {profile.displayName}
              </h1>
              
              <div className="font-mono text-sm md:text-base text-zinc-400 dark:text-zinc-500 mb-3.5 flex flex-wrap items-center gap-2.5">
                <span className="font-bold text-zinc-500 dark:text-zinc-400">Handle:</span>
                <span className="text-black dark:text-white font-semibold">@{usernameKey}</span>
              </div>

              <div className="flex flex-wrap items-center gap-2 mb-4">
                <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400 bg-zinc-200/50 dark:bg-zinc-900/50">
                  {profile.role}
                </span>
                <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400 flex items-center gap-1">
                  <MapPin className="w-2.5 h-2.5" />
                  {profile.location}
                </span>
              </div>

              <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm leading-relaxed max-w-2xl">
                {profile.bio}
              </p>

              {/* Social Counts & Github Link */}
              <div className="flex flex-wrap items-center gap-6 mt-6 pt-6 border-t border-dashed border-zinc-200 dark:border-zinc-800">
                <button 
                  onClick={() => setIsFollowersModalOpen(true)}
                  className="flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 hover:text-black dark:hover:text-white transition-colors cursor-pointer"
                >
                  <Users className="w-3.5 h-3.5" />
                  <span className="font-bold text-black dark:text-white">
                    {isSelf ? followersList.length : profile.followersCount}
                  </span> followers
                </button>
                
                <button 
                  onClick={() => setIsFollowingModalOpen(true)}
                  className="flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 hover:text-black dark:hover:text-white transition-colors cursor-pointer"
                >
                  <User className="w-3.5 h-3.5" />
                  <span className="font-bold text-black dark:text-white">
                    {isSelf ? followingList.length : profile.followingCount}
                  </span> following
                </button>

                <div className="flex flex-wrap items-center gap-4 ml-auto sm:ml-0">
                  <a 
                    href={`https://github.com/${usernameKey}`} 
                    target="_blank" 
                    rel="noopener noreferrer" 
                    className="flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 hover:text-blue-500 dark:hover:text-blue-400 transition-colors"
                  >
                    <GithubIcon className="w-3.5 h-3.5" />
                    <span>@{usernameKey}</span>
                  </a>

                  {profile.twitter && (
                    <a 
                      href={`https://x.com/${profile.twitter}`} 
                      target="_blank" 
                      rel="noopener noreferrer" 
                      className="flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 hover:text-blue-500 dark:hover:text-blue-400 transition-colors"
                    >
                      <TwitterIcon className="w-3.5 h-3.5" />
                      <span>@{profile.twitter}</span>
                    </a>
                  )}

                  {profile.linkedin && (
                    <a 
                      href={`https://linkedin.com/in/${profile.linkedin}`} 
                      target="_blank" 
                      rel="noopener noreferrer" 
                      className="flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 hover:text-blue-500 dark:hover:text-blue-400 transition-colors"
                    >
                      <LinkedinIcon className="w-3.5 h-3.5" />
                      <span>@{profile.linkedin}</span>
                    </a>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Action Button: Edit or Follow */}
          <div className="flex-shrink-0">
            {isSelf ? (
              <button
                onClick={() => setIsEditModalOpen(true)}
                className="flex items-center justify-center gap-2 px-4 h-9 bg-black text-white dark:bg-white dark:text-black hover:bg-zinc-800 dark:hover:bg-zinc-100 border border-zinc-850 dark:border-zinc-100 font-mono text-[10px] uppercase font-bold tracking-wider cursor-pointer transition-all duration-150 rounded-none w-full md:w-auto"
              >
                <Edit3 className="w-3.5 h-3.5" />
                Edit Profile
              </button>
            ) : (
              <button
                onClick={handleFollowToggle}
                className={`flex items-center justify-center gap-2 px-5 h-9 font-mono text-[10px] uppercase font-bold tracking-wider cursor-pointer transition-all duration-150 rounded-none w-full md:w-auto border ${
                  isFollowingThisUser
                    ? "bg-[#f6f6f3] text-black border-zinc-400 dark:bg-[#171616] dark:text-white dark:border-zinc-850 hover:bg-zinc-200/50 dark:hover:bg-zinc-900/50"
                    : "bg-blue-600 text-white border-blue-600 hover:bg-blue-700 hover:border-blue-700"
                }`}
              >
                {isFollowingThisUser ? (
                  <>
                    <Check className="w-3.5 h-3.5" />
                    Following
                  </>
                ) : (
                  <>
                    <Users className="w-3.5 h-3.5" />
                    Follow
                  </>
                )}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Tabs navigation */}
      <div className="flex border-b border-zinc-200 dark:border-zinc-800 mb-8 font-mono text-[10px] uppercase font-bold tracking-wide">
        <button
          onClick={() => setActiveTab("recipes")}
          className={`px-4 py-3 border-b-2 -mb-[2px] transition-all flex items-center gap-1.5 cursor-pointer ${
            activeTab === "recipes"
              ? "border-blue-500 text-blue-600 dark:text-blue-400 bg-zinc-100/30 dark:bg-zinc-900/10"
              : "border-transparent text-zinc-400 hover:text-black dark:hover:text-white"
          }`}
        >
          Published ({userRecipes.length})
        </button>
        <button
          onClick={() => setActiveTab("starred")}
          className={`px-4 py-3 border-b-2 -mb-[2px] transition-all flex items-center gap-1.5 cursor-pointer ${
            activeTab === "starred"
              ? "border-blue-500 text-blue-600 dark:text-blue-400 bg-zinc-100/30 dark:bg-zinc-900/10"
              : "border-transparent text-zinc-400 hover:text-black dark:hover:text-white"
          }`}
        >
          <Star className="w-3 h-3" />
          Starred ({isSelf ? userStarredRecipes.length : 2})
        </button>
      </div>

      {/* Tab Contents */}
      <div className="animate-fade-in">
        
        {/* RECIPES TAB */}
        {activeTab === "recipes" && (
          <div className="flex flex-col gap-6">
            {userRecipes.length === 0 ? (
              <div className="w-full border border-dashed border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/30 dark:bg-[#171616]/30 py-16 text-center">
                <p className="font-mono text-[10px] text-zinc-400 dark:text-zinc-500">No published configs found.</p>
              </div>
            ) : (
              userRecipes.map((recipe) => (
                <RecipeCard 
                  key={recipe.id} 
                  recipe={recipe} 
                  copiedId={copiedId} 
                  handleCopy={handleCopy} 
                  starredRecipeIds={starredRecipeIds}
                  toggleStar={toggleStar}
                  router={router}
                />
              ))
            )}
          </div>
        )}

        {/* STARRED TAB */}
        {activeTab === "starred" && (
          <div className="flex flex-col gap-6">
            {userStarredRecipes.length === 0 ? (
              <div className="w-full border border-dashed border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/30 dark:bg-[#171616]/30 py-16 text-center">
                <p className="font-mono text-[10px] text-zinc-400 dark:text-zinc-500">No starred recipes yet.</p>
              </div>
            ) : (
              userStarredRecipes.map((recipe) => (
                <RecipeCard 
                  key={recipe.id} 
                  recipe={recipe} 
                  copiedId={copiedId} 
                  handleCopy={handleCopy} 
                  starredRecipeIds={starredRecipeIds}
                  toggleStar={toggleStar}
                  router={router}
                />
              ))
            )}
          </div>
        )}

      </div>

      {/* EDIT PROFILE MODAL */}
      {isEditModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
          <div className="relative w-full max-w-lg border border-zinc-450 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-6 shadow-2xl rounded-none">
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

            <div className="flex items-center justify-between border-b border-zinc-200 dark:border-zinc-800 pb-4 mb-4">
              <h3 className="font-switzer font-bold text-lg text-black dark:text-white">Edit Profile Bio</h3>
              <button 
                onClick={() => setIsEditModalOpen(false)}
                className="text-zinc-400 hover:text-black dark:hover:text-white transition-colors cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleSaveProfile} className="space-y-4">
              <div>
                <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">Display Name</label>
                <input
                  type="text"
                  required
                  value={editForm.displayName}
                  onChange={(e) => setEditForm(prev => ({ ...prev, displayName: e.target.value }))}
                  className="w-full h-9 px-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-sm font-switzer outline-none focus:border-blue-500 rounded-none"
                />
              </div>

              <div>
                <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">Location</label>
                <input
                  type="text"
                  value={editForm.location}
                  onChange={(e) => setEditForm(prev => ({ ...prev, location: e.target.value }))}
                  className="w-full h-9 px-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-sm font-switzer outline-none focus:border-blue-500 rounded-none"
                />
              </div>


              <div>
                <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">Twitter Username</label>
                <input
                  type="text"
                  placeholder="e.g. janesmith"
                  value={editForm.twitter}
                  onChange={(e) => setEditForm(prev => ({ ...prev, twitter: e.target.value }))}
                  className="w-full h-9 px-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-sm font-mono outline-none focus:border-blue-500 rounded-none"
                />
              </div>

              <div>
                <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">LinkedIn Username</label>
                <input
                  type="text"
                  placeholder="e.g. janesmith"
                  value={editForm.linkedin}
                  onChange={(e) => setEditForm(prev => ({ ...prev, linkedin: e.target.value }))}
                  className="w-full h-9 px-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-sm font-mono outline-none focus:border-blue-500 rounded-none"
                />
              </div>

              <div>
                <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500 mb-1.5">Profile Bio</label>
                <textarea
                  rows={3}
                  value={editForm.bio}
                  onChange={(e) => setEditForm(prev => ({ ...prev, bio: e.target.value }))}
                  className="w-full p-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-sm font-switzer outline-none focus:border-blue-500 rounded-none resize-none"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-4 border-t border-zinc-200 dark:border-zinc-800">
                <button
                  type="button"
                  onClick={() => setIsEditModalOpen(false)}
                  className="px-4 h-9 font-mono text-[10px] uppercase font-bold tracking-wider border border-zinc-300 dark:border-zinc-800 text-zinc-500 hover:text-black dark:hover:text-white hover:bg-zinc-100 dark:hover:bg-zinc-900 transition-colors rounded-none cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-5 h-9 font-mono text-[10px] uppercase font-bold tracking-wider bg-blue-600 text-white hover:bg-blue-700 transition-colors rounded-none cursor-pointer"
                >
                  Save Changes
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* FOLLOWERS MODAL */}
      {isFollowersModalOpen && (
        <UsersListModal 
          title="Followers"
          list={isSelf ? followersList : ["alice", "qwen", "mistral"]}
          isSelf={isSelf}
          onRemove={isSelf ? async (username) => {
            if (!supabase || !user) return;
            
            const followerProfile = profiles[username.toLowerCase()];
            const followerAuthId = followerProfile?.authId;
            
            if (!followerAuthId) {
              try {
                const { data } = await supabase
                  .from("profiles")
                  .select("auth_id")
                  .eq("username", username)
                  .maybeSingle();
                if (data) {
                  const { error } = await supabase
                    .from("follows")
                    .delete()
                    .eq("follower_id", data.auth_id)
                    .eq("following_id", user.id);
                  if (error) throw error;
                  setFollowersList(prev => prev.filter(u => u !== username));
                  toast.success(`Removed @${username} from followers`);
                  return;
                }
              } catch (e) {}
              toast.error("Failed to remove follower");
              return;
            }

            try {
              const { error } = await supabase
                .from("follows")
                .delete()
                .eq("follower_id", followerAuthId)
                .eq("following_id", user.id);
              if (error) throw error;
              
              setFollowersList(prev => prev.filter(u => u !== username));
              toast.success(`Removed @${username} from followers`);
            } catch (err: any) {
              toast.error("Failed to remove follower", {
                description: err.message
              });
            }
          } : undefined}
          onClose={() => setIsFollowersModalOpen(false)}
          profiles={profiles}
          onNavigate={(u) => {
            setIsFollowersModalOpen(false);
            router.push(`/users/${u}`);
          }}
        />
      )}

      {/* FOLLOWING MODAL */}
      {isFollowingModalOpen && (
        <UsersListModal 
          title="Following"
          list={isSelf ? followingList : ["alice", "google"]}
          isSelf={isSelf}
          onRemove={isSelf ? async (username) => {
            if (!supabase || !user) return;
            
            const followingProfile = profiles[username.toLowerCase()];
            const followingAuthId = followingProfile?.authId;
            
            if (!followingAuthId) {
              try {
                const { data } = await supabase
                  .from("profiles")
                  .select("auth_id")
                  .eq("username", username)
                  .maybeSingle();
                if (data) {
                  const { error } = await supabase
                    .from("follows")
                    .delete()
                    .eq("follower_id", user.id)
                    .eq("following_id", data.auth_id);
                  if (error) throw error;
                  setFollowingList(prev => prev.filter(u => u !== username));
                  toast.success(`Unfollowed @${username}`);
                  return;
                }
              } catch (e) {}
              toast.error("Failed to unfollow");
              return;
            }

            try {
              const { error } = await supabase
                .from("follows")
                .delete()
                .eq("follower_id", user.id)
                .eq("following_id", followingAuthId);
              if (error) throw error;
              
              setFollowingList(prev => prev.filter(u => u !== username));
              toast.success(`Unfollowed @${username}`);
            } catch (err: any) {
              toast.error("Failed to unfollow", {
                description: err.message
              });
            }
          } : undefined}
          onClose={() => setIsFollowingModalOpen(false)}
          profiles={profiles}
          onNavigate={(u) => {
            setIsFollowingModalOpen(false);
            router.push(`/users/${u}`);
          }}
        />
      )}

    </div>
  );
}

// Sub-Component for modular cleanly rendered Recipe Cards
function RecipeCard({ 
  recipe, 
  copiedId, 
  handleCopy, 
  starredRecipeIds, 
  toggleStar,
  router 
}: { 
  recipe: Recipe; 
  copiedId: string | null; 
  handleCopy: (e: React.MouseEvent, id: string) => void;
  starredRecipeIds: string[];
  toggleStar: (e: React.MouseEvent, id: string) => void;
  router: any;
}) {
  const isStarred = starredRecipeIds.includes(recipe.id);

  return (
    <div 
      onClick={() => router.push(`/recipes/${recipe.id}`)}
      className="group relative flex flex-col justify-between border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-6 rounded-none transition-colors min-h-[220px] cursor-pointer hover:bg-zinc-200/30 dark:hover:bg-zinc-900/30"
    >
      {/* SVG Corner L-Brackets */}
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
            <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400">
              {recipe.hardware.targetPlatform.toUpperCase()}
            </span>
            <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 bg-zinc-200 dark:bg-zinc-900 text-zinc-850 dark:text-zinc-200">
              {recipe.hardware.minVram} VRAM
            </span>
          </div>
          <h2 className="text-xl font-semibold tracking-tight font-switzer text-black dark:text-white">
            {recipe.creator}/<span className="text-blue-600 dark:text-blue-400">{recipe.name}</span>
          </h2>
        </div>

        {/* Dynamic Star Button */}
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
        <div className="font-mono text-xs text-zinc-400 dark:text-zinc-500">
          <span className="font-bold text-zinc-500 dark:text-zinc-400">Base Model:</span> {recipe.baseModel}
        </div>
      </div>

      {/* Deployment copy row */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mt-6">
        <div className="flex-1 min-w-0 w-full sm:max-w-xl">
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

        {/* Telemetry statistics */}
        <div className="flex items-center gap-1.5 font-mono text-[10px] text-zinc-500 dark:text-zinc-400 select-none shrink-0 self-end sm:self-auto">
          <Download className="w-3.5 h-3.5" />
          <span>{recipe.telemetry.runs} runs</span>
        </div>
      </div>
    </div>
  );
}

// Sub-Component: Followers / Following List Modal
function UsersListModal({ 
  title, 
  list,
  isSelf,
  onRemove,
  onClose, 
  profiles,
  onNavigate 
}: { 
  title: string; 
  list: string[];
  isSelf: boolean;
  onRemove?: (username: string) => void;
  onClose: () => void; 
  profiles: any;
  onNavigate: (username: string) => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="relative w-full max-w-md border border-zinc-450 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-6 shadow-2xl rounded-none">
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

        <div className="flex items-center justify-between border-b border-zinc-200 dark:border-zinc-800 pb-4 mb-4">
          <h3 className="font-switzer font-bold text-base text-black dark:text-white">{title}</h3>
          <button 
            onClick={onClose}
            className="text-zinc-400 hover:text-black dark:hover:text-white transition-colors cursor-pointer"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="max-h-[300px] overflow-y-auto space-y-3 pr-1">
          {list.length === 0 ? (
            <div className="text-center py-8 font-mono text-[10px] text-zinc-400">
              No builders found in this list.
            </div>
          ) : (
            list.map((username) => {
              const userProfile = profiles[username] || {
                displayName: username,
                role: "Contributor"
              };
              return (
                <div 
                  key={username}
                  onClick={() => onNavigate(username)}
                  className="flex items-center justify-between p-3 border border-zinc-300 dark:border-zinc-850 hover:bg-zinc-200/40 dark:hover:bg-zinc-900/40 transition-colors cursor-pointer group"
                >
                  <div>
                    <h4 className="font-switzer font-semibold text-sm text-black dark:text-white group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors">
                      {userProfile.displayName || username}
                    </h4>
                    <div className="font-mono text-[9px] text-zinc-400 mt-0.5">
                      @{username} • {userProfile.role || "Contributor"}
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                    {onRemove ? (
                      title === "Followers" ? (
                        <button
                          onClick={() => onRemove(username)}
                          title="Remove Follower"
                          className="p-1.5 border border-zinc-300 dark:border-zinc-800 text-zinc-400 hover:text-red-500 hover:border-red-500/30 hover:bg-red-500/5 rounded transition-all cursor-pointer"
                        >
                          <X className="w-3.5 h-3.5" />
                        </button>
                      ) : (
                        <button
                          onClick={() => onRemove(username)}
                          className="px-2.5 h-6 border border-red-500/20 bg-red-500/5 hover:bg-red-500/15 text-red-650 dark:text-red-400 font-mono text-[9px] uppercase font-bold tracking-wider rounded-none cursor-pointer transition-all active:scale-[0.97]"
                        >
                          Unfollow
                        </button>
                      )
                    ) : (
                      <ArrowRight className="w-3.5 h-3.5 text-zinc-400 group-hover:text-black dark:group-hover:text-white group-hover:translate-x-0.5 transition-all" />
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
