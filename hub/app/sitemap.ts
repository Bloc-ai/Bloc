import { MetadataRoute } from "next";
import { getSupabaseAnon } from "@/lib/supabase-server";
import { registryRecipes } from "@/lib/registry-data";

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const baseUrl = "https://bloc-theta.vercel.app";

  // Static routes of the web application
  const staticPaths = [
    "",
    "/registry",
    "/installation",
    "/feed",
    "/blog"
  ];

  const staticRoutes = staticPaths.map((path) => ({
    url: `${baseUrl}${path}`,
    lastModified: new Date(),
    changeFrequency: "daily" as const,
    priority: path === "" ? 1.0 : 0.8,
  }));

  // Fetch all custom recipes from Supabase to add to sitemap
  const supabase = getSupabaseAnon();
  let recipeRoutes: any[] = [];
  let userRoutes: any[] = [];

  if (supabase) {
    try {
      // 1. Fetch recipes
      const { data: dbRecipes } = await supabase
        .from("recipes")
        .select("creator, name, created_at")
        .limit(1000);

      if (dbRecipes) {
        recipeRoutes = dbRecipes.map((row: any) => ({
          url: `${baseUrl}/recipes/${row.creator}/${row.name}`,
          lastModified: row.created_at ? new Date(row.created_at) : new Date(),
          changeFrequency: "weekly" as const,
          priority: 0.7,
        }));
      }

      // 2. Fetch users
      const { data: dbProfiles } = await supabase
        .from("profiles")
        .select("username, updated_at")
        .limit(1000);

      if (dbProfiles) {
        userRoutes = dbProfiles.map((row: any) => ({
          url: `${baseUrl}/users/${row.username}`,
          lastModified: row.updated_at ? new Date(row.updated_at) : new Date(),
          changeFrequency: "weekly" as const,
          priority: 0.6,
        }));
      }
    } catch (e) {
      console.error("Error generating dynamic sitemap routes:", e);
    }
  }

  // Add static registryRecipes (mock recipes)
  const mockRecipes = registryRecipes.map((row) => ({
    url: `${baseUrl}/recipes/${row.creator}/${row.name}`,
    lastModified: new Date(),
    changeFrequency: "weekly" as const,
    priority: 0.7,
  }));

  // Deduplicate and combine all
  const allRoutes = [
    ...staticRoutes,
    ...mockRecipes,
    ...recipeRoutes.filter(r => !mockRecipes.some(mr => mr.url === r.url)),
    ...userRoutes,
  ];

  return allRoutes;
}
