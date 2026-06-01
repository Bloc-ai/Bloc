import { registryRecipes, Recipe } from "@/lib/registry-data";
import { getSupabaseAnon } from "@/lib/supabase-server";
import UserProfileClient from "./UserProfileClient";
import type { Metadata } from "next";

export const revalidate = 60; // Revalidate user profile cache every 60 seconds

interface PageProps {
  params: Promise<{
    username: string;
  }>;
}

async function getDbProfile(usernameKey: string) {
  const supabase = getSupabaseAnon();
  if (!supabase) return null;
  
  try {
    const { data, error } = await supabase
      .from("profiles")
      .select("*")
      .eq("username", usernameKey)
      .maybeSingle();

    if (error) throw error;
    return data;
  } catch (err) {
    console.error("Error loading profile on server:", err);
    return null;
  }
}

async function getDbRecipes(usernameKey: string): Promise<Recipe[]> {
  const supabase = getSupabaseAnon();
  if (!supabase) return [];
  try {
    const { data, error } = await supabase
      .from("recipes")
      .select("id, name, creator, description, base_model, min_vram, target_platform, yaml_content, compat_builds, created_at, telemetry_events(count)")
      .eq("creator", usernameKey)
      .order("created_at", { ascending: false })
      .limit(100);

    if (error || !data) return [];

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
    console.error("Error loading recipes on server profile:", e);
    return [];
  }
}

async function getFollowCounts(authId: string) {
  const supabase = getSupabaseAnon();
  if (!supabase) return { followersCount: 0, followingCount: 0 };
  
  try {
    const [followersCountRes, followingCountRes] = await Promise.all([
      supabase.from("follows").select("follower_id", { count: "exact", head: true }).eq("following_id", authId),
      supabase.from("follows").select("following_id", { count: "exact", head: true }).eq("follower_id", authId)
    ]);
    return {
      followersCount: followersCountRes.count || 0,
      followingCount: followingCountRes.count || 0
    };
  } catch (e) {
    console.error("Error loading follow counts on server:", e);
    return { followersCount: 0, followingCount: 0 };
  }
}

async function getFollowLists(authId: string) {
  const supabase = getSupabaseAnon();
  if (!supabase) return { followers: [], following: [], profilesMap: {} };
  try {
    // Fetch followers with profiles
    const { data: followersData } = await supabase
      .from("follows")
      .select(`
        follower_id,
        profiles:follower_id (auth_id, username, display_name, avatar_url, role, location, bio)
      `)
      .eq("following_id", authId);

    // Fetch following with profiles
    const { data: followingData } = await supabase
      .from("follows")
      .select(`
        following_id,
        profiles:following_id (auth_id, username, display_name, avatar_url, role, location, bio)
      `)
      .eq("follower_id", authId);

    const followers: string[] = [];
    const following: string[] = [];
    const profilesMap: Record<string, any> = {};

    if (followersData) {
      followersData.forEach((f: any) => {
        const p = f.profiles;
        if (p && p.username) {
          followers.push(p.username);
          profilesMap[p.username.toLowerCase()] = {
            displayName: p.display_name || p.username,
            role: p.role || "Contributor",
            avatarUrl: p.avatar_url || "",
            github: p.username,
            authId: p.auth_id,
            location: p.location || "Unknown Location",
            bio: p.bio || ""
          };
        }
      });
    }

    if (followingData) {
      followingData.forEach((f: any) => {
        const p = f.profiles;
        if (p && p.username) {
          following.push(p.username);
          profilesMap[p.username.toLowerCase()] = {
            displayName: p.display_name || p.username,
            role: p.role || "Contributor",
            avatarUrl: p.avatar_url || "",
            github: p.username,
            authId: p.auth_id,
            location: p.location || "Unknown Location",
            bio: p.bio || ""
          };
        }
      });
    }

    return { followers, following, profilesMap };
  } catch (e) {
    console.error("Error loading follow lists on server:", e);
    return { followers: [], following: [], profilesMap: {} };
  }
}

async function getStarredRecipeIds(authId: string): Promise<string[]> {
  const supabase = getSupabaseAnon();
  if (!supabase) return [];
  try {
    const { data, error } = await supabase
      .from("stars")
      .select("recipe_id")
      .eq("user_id", authId);
    if (error || !data) return [];
    return data.map((s: any) => s.recipe_id);
  } catch (e) {
    console.error("Error loading starred recipe IDs on server:", e);
    return [];
  }
}

async function getRecipesForIds(recipeIds: string[]): Promise<Recipe[]> {
  const supabase = getSupabaseAnon();
  if (!supabase || recipeIds.length === 0) return [];
  
  try {
    const dbIds = recipeIds.filter(id => !registryRecipes.some(r => r.id === id));
    if (dbIds.length === 0) return [];
    
    const orParts = dbIds.map(id => {
      const parts = id.split('/');
      if (parts.length < 2) return null;
      const creator = parts[0];
      const name = parts[1];
      return `and(creator.eq.${creator},name.eq.${name})`;
    }).filter(Boolean);
    
    if (orParts.length === 0) return [];
    const orQuery = orParts.join(',');
    
    const { data, error } = await supabase
      .from("recipes")
      .select("id, name, creator, description, base_model, min_vram, target_platform, yaml_content, compat_builds, created_at, telemetry_events(count)")
      .or(orQuery);
      
    if (error || !data) return [];
    
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
    console.error("Error loading recipes for starred IDs on server:", e);
    return [];
  }
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const params = await props.params;
  const username = params.username || "";
  const usernameKey = username.toLowerCase();
  
  const profile = await getDbProfile(usernameKey);
  const displayName = profile?.display_name || username;
  const bio = profile?.bio || "Local AI developer and registry contributor.";
  
  return {
    title: `@${username} (${displayName}) - Bloc Hub`,
    description: bio,
  };
}

export default async function UserProfilePage(props: PageProps) {
  const params = await props.params;
  const rawUsername = params.username || "";
  const usernameKey = rawUsername.toLowerCase();

  const dbProfile = await getDbProfile(usernameKey);
  const profile = dbProfile ? {
    displayName: dbProfile.display_name || rawUsername,
    bio: dbProfile.bio || "Local AI developer and registry contributor.",
    location: dbProfile.location || "Unknown Location",
    github: dbProfile.username,
    twitter: dbProfile.twitter || "",
    linkedin: dbProfile.linkedin || "",
    avatarUrl: `https://github.com/${dbProfile.username}.png`,
    role: dbProfile.role || "Contributor",
    followersCount: 0,
    followingCount: 0,
    authId: dbProfile.auth_id
  } : {
    displayName: rawUsername,
    bio: "Local AI developer and registry contributor.",
    location: "Unknown Location",
    github: rawUsername,
    twitter: "",
    linkedin: "",
    avatarUrl: `https://github.com/${usernameKey}.png`,
    role: "Contributor",
    followersCount: 0,
    followingCount: 0,
    authId: null
  };

  let followers: string[] = [];
  let following: string[] = [];
  let profilesMap: Record<string, any> = {};

  if (profile.authId) {
    const counts = await getFollowCounts(profile.authId);
    profile.followersCount = counts.followersCount;
    profile.followingCount = counts.followingCount;
    
    const lists = await getFollowLists(profile.authId);
    followers = lists.followers;
    following = lists.following;
    profilesMap = lists.profilesMap;
  }

  // Recipes owned by this profile (Mock recipes + Supabase recipes)
  const dbRecipes = await getDbRecipes(usernameKey);
  const userRecipes = [
    ...registryRecipes.filter((r) => r.creator.toLowerCase() === usernameKey),
    ...dbRecipes.filter((dbR) => dbR.creator.toLowerCase() === usernameKey && !registryRecipes.some((mockR) => mockR.id === dbR.id))
  ];

  // Starred recipes list (Mock recipes + Supabase recipes)
  let profileStarredIds: string[] = [];
  if (profile.authId) {
    profileStarredIds = await getStarredRecipeIds(profile.authId);
  }
  
  const dbStarredRecipes = profileStarredIds.length > 0 ? await getRecipesForIds(profileStarredIds) : [];
  
  const userStarredRecipes = [
    ...registryRecipes.filter((r) => profileStarredIds.includes(r.id)),
    ...dbStarredRecipes.filter((dbR) => profileStarredIds.includes(dbR.id) && !registryRecipes.some((mockR) => mockR.id === dbR.id))
  ];

  // Pre-seed profiles map with the target profile details
  profilesMap[usernameKey] = profile;

  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "ProfilePage",
    "mainEntity": {
      "@type": "Person",
      "name": profile.displayName,
      "alternateName": usernameKey,
      "description": profile.bio,
      "homeLocation": {
        "@type": "Place",
        "name": profile.location
      },
      "agentInteractionStatistic": {
        "@type": "InteractionCounter",
        "interactionType": "https://schema.org/FollowAction",
        "userInteractionCount": profile.followersCount
      }
    }
  };

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <UserProfileClient
        usernameKey={usernameKey}
        rawUsername={rawUsername}
        initialProfile={profile}
        initialRecipes={userRecipes}
        initialStarredRecipes={userStarredRecipes}
        initialFollowers={followers}
        initialFollowing={following}
        initialIsFollowing={false}
        initialViewerStarredIds={[]}
        initialProfilesMap={profilesMap}
        isSelf={false}
      />
    </>
  );
}
