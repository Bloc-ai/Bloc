"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { supabase } from "@/lib/supabase";

export type User = {
  id: string;
  username: string;
  avatar_url: string;
};

type AuthContextType = {
  user: User | null;
  loading: boolean;
  login: () => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<AuthContextType>({
  user: null,
  loading: true,
  login: async () => {},
  logout: () => {},
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    if (!supabase) {
      // C3 Fix: No silent localStorage fallback in any environment.
      // If Supabase is not configured, simply treat the user as unauthenticated.
      console.warn(
        "[AuthContext] Supabase is not configured. Running in unauthenticated mode."
      );
      setUser(null);
      setLoading(false);
      return;
    }

    const handleSession = async (session: any) => {
      if (typeof window !== "undefined") {
        if (session?.access_token) {
          const isSecure = window.location.protocol === "https:";
          document.cookie = `sb-access-token=${session.access_token}; path=/; max-age=${session.expires_in || 3600}; SameSite=Lax${isSecure ? "; Secure" : ""}`;
        } else {
          const isSecure = window.location.protocol === "https:";
          document.cookie = `sb-access-token=; path=/; expires=Thu, 01 Jan 1970 00:00:00 UTC; SameSite=Lax${isSecure ? "; Secure" : ""}`;
        }
      }

      if (!session?.user) {
        setUser(null);
        setLoading(false);
        return;
      }

      // Fetch Native Profile to see if they've completed onboarding
      const { data: profile } = await supabase!
        .from("profiles")
        .select("username")
        .eq("auth_id", session.user.id)
        .maybeSingle();

      if (!profile) {
        // Native profile missing! Enforce Onboarding to claim handle.
        const githubUsername =
          session.user.user_metadata.user_name ||
          session.user.user_metadata.preferred_username ||
          "";
        setUser({
          id: session.user.id,
          username: githubUsername,
          avatar_url: session.user.user_metadata.avatar_url || "",
        });

        if (
          window.location.pathname !== "/onboarding" &&
          window.location.pathname !== "/login"
        ) {
          router.push("/onboarding");
        }
      } else {
        // Profile exists! Complete login.
        setUser({
          id: session.user.id,
          username: profile.username,
          avatar_url:
            session.user.user_metadata.avatar_url ||
            `https://github.com/${profile.username}.png`,
        });
      }
      setLoading(false);
    };

    const {
      data: { subscription },
    } = supabase.auth.onAuthStateChange((_event, session) => {
      handleSession(session);
    });

    return () => {
      subscription.unsubscribe();
    };
  }, []);

  const login = async () => {
    if (!supabase) {
      toast.error("Authentication unavailable", {
        description:
          "Supabase is not configured. Please set up your environment variables.",
      });
      return;
    }

    // Trigger real OAuth Redirect with GitHub
    await supabase.auth.signInWithOAuth({
      provider: "github",
      options: {
        redirectTo: `${window.location.origin}/`,
      },
    });
  };

  const logout = async () => {
    if (supabase) {
      await supabase.auth.signOut();
      toast.info("Logged out of GitHub securely");
    } else {
      setUser(null);
      toast.info("Logged out successfully");
    }
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => useContext(AuthContext);
