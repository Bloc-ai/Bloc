import { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: [
        "/api/",
        "/auth/",
        "/onboarding/",
        "/login",
      ],
    },
    sitemap: "https://bloc-theta.vercel.app/sitemap.xml",
  };
}
