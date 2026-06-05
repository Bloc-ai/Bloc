"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import Link from "next/link";
import Image from "next/image";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { useSearchContext } from "fumadocs-ui/contexts/search";
import { Search, User, Cpu, BookOpen, CornerDownRight, Command } from "lucide-react";
import { supabase } from "@/lib/supabase";

export function ArrowIcon() {
  return (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M1 9L9 1M9 1H1M9 1V9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
}

export function CTAButton({ 
  label, 
  className = "", 
  variant = "small",
  href,
  type = "button",
  onClick,
  disabled = false
}: { 
  label: string; 
  className?: string;
  variant?: "small" | "large";
  href?: string;
  type?: "button" | "submit" | "reset";
  onClick?: () => void;
  disabled?: boolean;
}) {
  const isLarge = variant === "large";
  
  const content = (
    <div className="flex items-center relative gap-0">
      <div className={`opacity-0 -translate-x-full group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300 flex items-center overflow-hidden ${
        isLarge ? "w-0 group-hover:w-4 group-hover:mr-3" : "w-0 group-hover:w-3 group-hover:mr-2"
      }`}>
        <ArrowIcon />
      </div>
      <span className="relative z-10">{label}</span>
      <div className={`opacity-100 translate-x-0 group-hover:translate-x-full group-hover:opacity-0 transition-all duration-300 flex items-center overflow-hidden ${
        isLarge ? "w-4 ml-3 group-hover:w-0 group-hover:ml-0" : "w-3 ml-2 group-hover:w-0 group-hover:ml-0"
      }`}>
        <ArrowIcon />
      </div>
    </div>
  );

  const styles = `group relative flex items-center justify-center transition-all duration-300 overflow-hidden pointer-events-auto bg-[#2563EB] text-white font-mono font-bold uppercase tracking-wider ${
    isLarge 
      ? "h-12 px-8 text-[13px] rounded-[14px]" 
      : "h-7 px-4 text-[10px] rounded-md hover:opacity-90"
  } ${disabled ? "opacity-50 cursor-not-allowed" : ""} ${className}`;

  if (href) {
    return (
      <Link href={href} className={styles}>
        {content}
      </Link>
    );
  }

  return (
    <button type={type} onClick={onClick} disabled={disabled} className={styles}>
      {content}
    </button>
  );
}

export default function Navbar() {
  const router = useRouter();
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const { user, logout } = useAuth();
  
  const pathname = usePathname();
  const isDocs = pathname?.startsWith("/docs");
  const { setOpenSearch } = useSearchContext();
  
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Global CMD+K Search Modal states
  const [isSearchOpen, setIsSearchOpen] = useState(false);
  const [globalSearchQuery, setGlobalSearchQuery] = useState("");
  const [dbRecipes, setDbRecipes] = useState<any[]>([]);
  const [dbProfiles, setDbProfiles] = useState<any[]>([]);

  // Traditional Navbar Search states
  const [isNavbarSearchOpen, setIsNavbarSearchOpen] = useState(false);
  const [navbarSearchQuery, setNavbarSearchQuery] = useState("");
  const navbarSearchRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsDropdownOpen(false);
      }
      if (navbarSearchRef.current && !navbarSearchRef.current.contains(event.target as Node)) {
        setIsNavbarSearchOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Listen for Cmd+K globally
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        if (isDocs) {
          setOpenSearch(true);
        } else {
          setIsSearchOpen((prev) => !prev);
        }
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isDocs, setOpenSearch]);

  // Load recipes and profiles from database when either search mode is active
  useEffect(() => {
    if (!isSearchOpen && !isNavbarSearchOpen) return;
    if (!supabase) return;

    async function loadSearchData() {
      if (!supabase) return;
      try {
        const { data: recs } = await supabase
          .from("recipes")
          .select("id, name, creator, description, base_model");
        if (recs) setDbRecipes(recs);

        const { data: profs } = await supabase
          .from("profiles")
          .select("username, display_name, avatar_url");
        if (profs) setDbProfiles(profs);
      } catch (err) {
        console.error("Error loading search metadata:", err);
      }
    }
    loadSearchData();
  }, [isSearchOpen, isNavbarSearchOpen]);

  // Filtering Logic for Global CMD+K Search
  const filteredModels = useMemo(() => {
    const q = globalSearchQuery.trim().toLowerCase();
    
    // Extract baseModels from dbRecipes
    const dbModels = dbRecipes.map(r => r.baseModel || r.base_model).filter(Boolean);
    const allModelsSet = new Set(dbModels);
    const allModels = Array.from(allModelsSet);

    if (!q) return allModels.slice(0, 6).map(m => {
      const parts = m.split("/");
      return { title: m, creator: parts[0] || "", name: parts[1] || m };
    });
    
    return allModels
      .filter(m => m.toLowerCase().includes(q))
      .map(m => {
        const parts = m.split("/");
        return {
          title: m,
          creator: parts[0] || "",
          name: parts[1] || m
        };
      })
      .slice(0, 6);
  }, [globalSearchQuery, dbRecipes]);

  const filteredRecipes = useMemo(() => {
    const q = globalSearchQuery.trim().toLowerCase();
    
    const allRecs = dbRecipes.map(r => ({
      id: `${r.creator}/${r.name}`,
      creator: r.creator,
      name: r.name,
      description: r.description || ""
    })).filter((v, i, self) => self.findIndex(t => t.id === v.id) === i);

    if (!q) return allRecs.slice(0, 5);

    return allRecs
      .filter(r => r.name.toLowerCase().includes(q) || r.creator.toLowerCase().includes(q) || r.description.toLowerCase().includes(q))
      .slice(0, 5);
  }, [globalSearchQuery, dbRecipes]);

  const filteredUsers = useMemo(() => {
    const q = globalSearchQuery.trim().toLowerCase();
    
    const allUsers = dbProfiles.map(p => ({
      username: p.username,
      display_name: p.display_name || p.username
    })).filter((v, i, self) => self.findIndex(t => t.username === v.username) === i);

    if (!q) return allUsers.slice(0, 4);

    return allUsers
      .filter(u => u.username.toLowerCase().includes(q) || (u.display_name && u.display_name.toLowerCase().includes(q)))
      .slice(0, 4);
  }, [globalSearchQuery, dbProfiles]);

  // Combine into a single indexable list for keyboard navigation
  const flatItems = useMemo(() => {
    const items: any[] = [];
    filteredModels.forEach(m => items.push({ type: "model", id: m.title, label: m.title, url: `/registry?search=${encodeURIComponent(m.title)}` }));
    filteredRecipes.forEach(r => items.push({ type: "recipe", id: r.id, label: r.id, url: `/recipes/${r.creator}/${r.name}` }));
    filteredUsers.forEach(u => items.push({ type: "user", id: u.username, label: `@${u.username}`, url: `/users/${u.username}` }));
    return items;
  }, [filteredModels, filteredRecipes, filteredUsers]);

  // Filtering Logic for Navbar Dropdown
  const navFilteredModels = useMemo(() => {
    const q = navbarSearchQuery.trim().toLowerCase();
    
    const dbModels = dbRecipes.map(r => r.baseModel || r.base_model).filter(Boolean);
    const allModelsSet = new Set(dbModels);
    const allModels = Array.from(allModelsSet);

    if (!q) return allModels.slice(0, 6).map(m => {
      const parts = m.split("/");
      return { title: m, creator: parts[0] || "", name: parts[1] || m };
    });
    
    return allModels
      .filter(m => m.toLowerCase().includes(q))
      .map(m => {
        const parts = m.split("/");
        return {
          title: m,
          creator: parts[0] || "",
          name: parts[1] || m
        };
      })
      .slice(0, 6);
  }, [navbarSearchQuery, dbRecipes]);

  const navFilteredRecipes = useMemo(() => {
    const q = navbarSearchQuery.trim().toLowerCase();
    
    const allRecs = dbRecipes.map(r => ({
      id: `${r.creator}/${r.name}`,
      creator: r.creator,
      name: r.name,
      description: r.description || ""
    })).filter((v, i, self) => self.findIndex(t => t.id === v.id) === i);

    if (!q) return allRecs.slice(0, 5);

    return allRecs
      .filter(r => r.name.toLowerCase().includes(q) || r.creator.toLowerCase().includes(q) || r.description.toLowerCase().includes(q))
      .slice(0, 5);
  }, [navbarSearchQuery, dbRecipes]);

  const navFilteredUsers = useMemo(() => {
    const q = navbarSearchQuery.trim().toLowerCase();
    
    const allUsers = dbProfiles.map(p => ({
      username: p.username,
      display_name: p.display_name || p.username
    })).filter((v, i, self) => self.findIndex(t => t.username === v.username) === i);

    if (!q) return allUsers.slice(0, 4);

    return allUsers
      .filter(u => u.username.toLowerCase().includes(q) || (u.display_name && u.display_name.toLowerCase().includes(q)))
      .slice(0, 4);
  }, [navbarSearchQuery, dbProfiles]);

  const navFlatItems = useMemo(() => {
    const items: any[] = [];
    navFilteredModels.forEach(m => items.push({ type: "model", id: m.title, label: m.title, url: `/registry?search=${encodeURIComponent(m.title)}` }));
    navFilteredRecipes.forEach(r => items.push({ type: "recipe", id: r.id, label: r.id, url: `/recipes/${r.creator}/${r.name}` }));
    navFilteredUsers.forEach(u => items.push({ type: "user", id: u.username, label: `@${u.username}`, url: `/users/${u.username}` }));
    return items;
  }, [navFilteredModels, navFilteredRecipes, navFilteredUsers]);

  const [navSelectedIndex, setNavSelectedIndex] = useState(0);

  // Reset index when navbar query changes
  useEffect(() => {
    setNavSelectedIndex(0);
  }, [navbarSearchQuery]);

  // Navigate helper for navbar inline dropdown
  const navigateNavTo = (url: string) => {
    setIsNavbarSearchOpen(false);
    setNavbarSearchQuery("");
    router.push(url);
  };

  // Keyboard navigation inside navbar dropdown
  useEffect(() => {
    if (!isNavbarSearchOpen || navFlatItems.length === 0) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setNavSelectedIndex(prev => (prev + 1) % navFlatItems.length);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setNavSelectedIndex(prev => (prev - 1 + navFlatItems.length) % navFlatItems.length);
      } else if (e.key === "Enter") {
        e.preventDefault();
        const activeItem = navFlatItems[navSelectedIndex];
        if (activeItem) {
          navigateNavTo(activeItem.url);
        }
      } else if (e.key === "Escape") {
        e.preventDefault();
        setIsNavbarSearchOpen(false);
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isNavbarSearchOpen, navSelectedIndex, navFlatItems]);

  const [selectedIndex, setSelectedIndex] = useState(0);

  // Reset index when query changes
  useEffect(() => {
    setSelectedIndex(0);
  }, [globalSearchQuery]);

  // Close search and navigate helper
  const navigateTo = (url: string) => {
    setIsSearchOpen(false);
    setGlobalSearchQuery("");
    router.push(url);
  };

  // Keyboard navigation inside modal
  useEffect(() => {
    if (!isSearchOpen || flatItems.length === 0) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setSelectedIndex(prev => (prev + 1) % flatItems.length);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setSelectedIndex(prev => (prev - 1 + flatItems.length) % flatItems.length);
      } else if (e.key === "Enter") {
        e.preventDefault();
        const activeItem = flatItems[selectedIndex];
        if (activeItem) {
          navigateTo(activeItem.url);
        }
      } else if (e.key === "Escape") {
        e.preventDefault();
        setIsSearchOpen(false);
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isSearchOpen, selectedIndex, flatItems]);

  const navItems = useMemo(() => {
    return user
      ? [
          { label: "Registry", href: "/registry" },
          { label: "Feed", href: "/feed" },
          { label: "Docs", href: "/docs" },
          { label: "Submit", href: "/registry/submit" },
        ]
      : [
          { label: "Registry", href: "/registry" },
          { label: "Installation", href: "/installation" },
          { label: "Updates", href: "/updates" },
          { label: "Docs", href: "/docs" },
          { label: "Blog", href: "/blog" },
        ];
  }, [user]);

  if (isDocs) {
    return (
      <nav 
        style={{ top: "var(--banner-height, 0px)" }}
        className="fixed left-0 right-0 h-12 z-50 bg-[#f6f6f3]/95 dark:bg-[#171616]/95 border-b border-zinc-200 dark:border-zinc-800 backdrop-blur-md pointer-events-auto select-none flex items-center px-4 md:px-6 justify-between gap-4 transition-all duration-300 ease-in-out"
      >
        {/* Brand */}
        <div className="flex items-center px-3 md:px-4 h-7 bg-[#2563EB] rounded-md shrink-0 shadow-sm">
          <Link href="/" className="font-mono text-[13px] font-medium leading-none text-white tracking-tight whitespace-nowrap">Bloc</Link>
        </div>

        {/* Search - perfectly centered in the viewport */}
        <div className="absolute left-1/2 -translate-x-1/2 w-full max-w-[200px] sm:max-w-xs md:max-w-md h-7 px-4 z-10">
          <div 
            onClick={() => setOpenSearch(true)}
            className="flex items-center h-full px-2.5 md:px-3 bg-black/5 dark:bg-white/5 border border-black/10 dark:border-white/10 rounded-md gap-1.5 md:gap-2 transition-all duration-300 hover:border-black/20 dark:hover:border-white/20 focus-within:border-blue-500 group/search cursor-pointer select-none"
          >
            <svg width="11" height="11" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className="text-black/40 dark:text-white/40 group-hover/search:text-black/60 dark:group-hover/search:text-white/60">
              <path d="M14.5 14.5L10.5 10.5M12.5 6.5C12.5 9.81371 9.81371 12.5 6.5 12.5C3.18629 12.5 0.5 9.81371 0.5 6.5C0.5 3.18629 3.18629 0.5 6.5 0.5C9.81371 0.5 12.5 3.18629 12.5 6.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
            <span className="flex-grow font-mono text-[9px] md:text-[10px] text-black/45 dark:text-white/45 text-left leading-none truncate">
              Search documentation...
            </span>
            <span className="hidden md:inline font-mono text-[8px] bg-black/10 dark:bg-white/10 text-black/40 dark:text-white/40 px-1.5 py-0.5 rounded leading-none select-none">
              ⌘K
            </span>
          </div>
        </div>

        {/* Navigation Items (Login) */}
        <div className="flex items-center gap-2 md:gap-3 shrink-0 relative z-20">

          {user ? (
            <div ref={dropdownRef} className="relative shrink-0">
              <button
                onClick={() => setIsDropdownOpen(!isDropdownOpen)}
                className="flex items-center gap-1.5 md:gap-2 h-7 px-1.5 md:px-2 bg-black/5 dark:bg-white/5 border border-black/10 dark:border-white/10 hover:border-black/20 dark:hover:border-white/20 rounded-md shrink-0 cursor-pointer transition-all duration-150 select-none outline-none font-mono text-[9px] md:text-[10px] font-bold text-black dark:text-white uppercase tracking-wider"
              >
                {user.avatar_url ? (
                  <Image src={user.avatar_url} alt="Avatar" width={16} height={16} className="w-3.5 h-3.5 md:w-4 md:h-4 rounded-full border border-black/10 dark:border-white/10 shrink-0" unoptimized />
                ) : (
                  <div className="w-3.5 h-3.5 md:w-4 md:h-4 rounded-full bg-blue-500 text-white flex items-center justify-center text-[7px] font-bold shrink-0">
                    {user.username.substring(0, 2).toUpperCase()}
                  </div>
                )}
                <span className="max-w-[70px] md:max-w-none truncate">{user.username}</span>
                <span className={`text-[5px] md:text-[6px] text-zinc-500 transition-transform duration-200 ${isDropdownOpen ? "rotate-180" : ""}`}>▼</span>
              </button>

              {isDropdownOpen && (
                <div className="absolute top-9 right-0 w-48 bg-[#f6f6f3]/95 dark:bg-[#171616]/95 border border-zinc-300 dark:border-zinc-800 text-black dark:text-white font-mono rounded-lg p-1 shadow-2xl backdrop-blur-xl z-50 select-none">
                  {/* SVG Corner L-Brackets */}
                  <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-y-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>

                  <div className="px-2.5 py-1.5 text-[8px] uppercase tracking-wider text-zinc-400 dark:text-zinc-550 font-bold border-b border-black/5 dark:border-white/5">
                    Developer Actions
                  </div>
                  <Link
                    href="/registry/submit"
                    onClick={() => setIsDropdownOpen(false)}
                    className="flex items-center justify-between text-[10px] px-3 py-2 rounded-md hover:bg-blue-500/10 hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer transition-colors uppercase tracking-wider font-bold"
                  >
                    Submit
                  </Link>

                  <div className="h-[1px] border-t border-dashed border-zinc-300 dark:border-zinc-800 my-1" />

                  <Link
                    href={`/users/${user.username}`}
                    onClick={() => setIsDropdownOpen(false)}
                    className="flex items-center justify-between text-[10px] px-3 py-2 rounded-md hover:bg-zinc-200/50 dark:hover:bg-zinc-900/50 cursor-pointer transition-colors text-zinc-650 dark:text-zinc-350 hover:text-black dark:hover:text-white uppercase tracking-wider font-bold"
                  >
                    Profile
                  </Link>


                  <div className="h-[1px] border-t border-dashed border-zinc-300 dark:border-zinc-800 my-1" />

                  <button
                    onClick={() => {
                      setIsDropdownOpen(false);
                      logout();
                    }}
                    className="w-full text-left flex items-center justify-between text-[10px] px-3 py-2 rounded-md hover:bg-red-500/10 hover:text-red-650 dark:hover:text-red-400 cursor-pointer transition-colors font-bold uppercase tracking-wider"
                  >
                    Sign out
                  </button>
                </div>
              )}
            </div>
          ) : (
            <CTAButton label="get started" href="/login" variant="small" className="shrink-0" />
          )}
        </div>
      </nav>
    );
  }

  return (
    <nav 
      style={{ top: "var(--banner-height, 0px)" }}
      className="fixed left-0 right-0 z-50 flex flex-col px-4 pt-2 gap-1 pointer-events-none transition-all duration-300 ease-in-out"
    >
      <div className="max-w-7xl w-full mx-auto hidden md:flex items-center h-10 gap-1 pointer-events-auto px-4 border-x border-transparent">
        <div className="flex items-center px-4 h-7 bg-[#2563EB] rounded-md shrink-0 shadow-sm">
          <Link href="/" className="font-mono text-[13px] font-medium leading-none text-white tracking-tight whitespace-nowrap">Bloc</Link>
        </div>
        
        {isDocs ? (
          <>
            {/* Interactive Docs Search Bar Trigger */}
            <div className="flex-1 relative h-7">
              <div 
                onClick={() => setOpenSearch(true)}
                className="flex items-center h-full px-3 bg-black/5 dark:bg-white/5 border border-black/10 dark:border-white/10 rounded-md gap-2 transition-all duration-300 hover:border-black/20 dark:hover:border-white/20 focus-within:border-blue-500 group/search cursor-pointer select-none"
              >
                <svg width="12" height="12" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className="text-black/40 dark:text-white/40 group-hover/search:text-black/60 dark:group-hover/search:text-white/60">
                  <path d="M14.5 14.5L10.5 10.5M12.5 6.5C12.5 9.81371 9.81371 12.5 6.5 12.5C3.18629 12.5 0.5 9.81371 0.5 6.5C0.5 3.18629 3.18629 0.5 6.5 0.5C9.81371 0.5 12.5 3.18629 12.5 6.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                <span className="flex-grow font-mono text-[10px] text-black/45 dark:text-white/45 text-left leading-none">
                  Search documentation...
                </span>
                <span className="font-mono text-[8px] bg-black/10 dark:bg-white/10 text-black/40 dark:text-white/40 px-1.5 py-0.5 rounded leading-none select-none">
                  ⌘K
                </span>
              </div>
            </div>

            {/* Submit Link ONLY for Docs Layout */}
            <Link
              href="/registry/submit"
              className="flex items-center flex-1 max-w-[120px] h-7 px-4 backdrop-blur-md border border-black/5 dark:border-white/5 bg-black/5 dark:bg-white/10 text-black/70 dark:text-white/80 rounded-md text-[10px] font-mono font-medium transition-all duration-300 group hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black"
            >
              <span className="flex-1 text-left truncate">Submit</span>
            </Link>
          </>
        ) : (
          <>
            {/* Interactive Search Bar (Traditional Dropdown style) */}
            <div className="flex-1 relative h-7 pointer-events-auto" ref={navbarSearchRef}>
              <div className="flex items-center h-full px-3 bg-black/5 dark:bg-white/5 border border-black/10 dark:border-white/10 rounded-md gap-2 transition-all duration-300 hover:border-black/20 dark:hover:border-white/20 focus-within:border-blue-500 group/search">
                <svg width="12" height="12" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className="text-black/40 dark:text-white/40 group-hover/search:text-black/60 dark:group-hover/search:text-white/60">
                  <path d="M14.5 14.5L10.5 10.5M12.5 6.5C12.5 9.81371 9.81371 12.5 6.5 12.5C3.18629 12.5 0.5 9.81371 0.5 6.5C0.5 3.18629 3.18629 0.5 6.5 0.5C9.81371 0.5 12.5 3.18629 12.5 6.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                <input
                  type="text"
                  placeholder="Search models, recipes, users..."
                  value={navbarSearchQuery}
                  onChange={(e) => {
                    setNavbarSearchQuery(e.target.value);
                    setIsNavbarSearchOpen(true);
                  }}
                  onFocus={() => setIsNavbarSearchOpen(true)}
                  className="w-full bg-transparent border-none outline-none font-mono text-[10px] text-black dark:text-white placeholder-black/40 dark:placeholder-white/40 leading-none h-full"
                />
                <span className="font-mono text-[8px] bg-black/10 dark:bg-white/10 text-black/40 dark:text-white/40 px-1.5 py-0.5 rounded leading-none select-none">
                  ⌘K
                </span>
              </div>

              {/* Inline Dropdown Panel */}
              {isNavbarSearchOpen && (
                <div className="absolute top-8 left-0 w-full min-w-[320px] max-w-[450px] bg-[#f6f6f3]/95 dark:bg-[#121212]/95 border border-zinc-300 dark:border-zinc-800 rounded-md shadow-2xl z-50 overflow-hidden flex flex-col font-mono max-h-[380px] backdrop-blur-xl">
                  {/* SVG Corner L-Brackets for premium brutalist look */}
                  <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2 h-2 fill-black dark:fill-white pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2 h-2 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-y-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>
                  <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none">
                    <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                  </svg>

                  <div className="flex-grow overflow-y-auto p-1.5 flex flex-col gap-1">
                    {navFlatItems.length === 0 ? (
                      <div className="py-8 text-center text-[10px] text-zinc-400">
                        No results found
                      </div>
                    ) : (
                      <>
                        {/* Models section */}
                        {navFilteredModels.length > 0 && (
                          <div className="flex flex-col gap-0.5 mb-1.5">
                            <div className="px-2 py-1 text-[8px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-widest border-b border-zinc-200/30 dark:border-zinc-800/30 pb-0.5 mb-0.5">
                              Models
                            </div>
                            {navFilteredModels.map((m) => {
                              const itemIdx = navFlatItems.findIndex(i => i.type === "model" && i.id === m.title);
                              const isSelected = navSelectedIndex === itemIdx;
                              return (
                                <button
                                  key={`nav-m-${m.title}`}
                                  onClick={() => navigateNavTo(navFlatItems[itemIdx].url)}
                                  onMouseEnter={() => setNavSelectedIndex(itemIdx)}
                                  className={`w-full px-2 py-1.5 text-left rounded-md flex items-center justify-between transition-all duration-100 text-[10px] ${
                                    isSelected 
                                      ? "bg-[#2563EB]/10 text-[#2563EB] dark:bg-[#2563EB]/25 dark:text-blue-400 font-semibold"
                                      : "text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200/40 dark:hover:bg-zinc-900/40"
                                  }`}
                                >
                                  <div className="flex items-center gap-1.5 truncate">
                                    <Cpu className="w-3 h-3 opacity-60" />
                                    <span className="truncate">{m.title}</span>
                                  </div>
                                  {isSelected && <CornerDownRight className="w-2.5 h-2.5 opacity-80" />}
                                </button>
                              );
                            })}
                          </div>
                        )}

                        {/* Recipes section */}
                        {navFilteredRecipes.length > 0 && (
                          <div className="flex flex-col gap-0.5 mb-1.5">
                            <div className="px-2 py-1 text-[8px] font-bold text-zinc-400 dark:text-zinc-550 uppercase tracking-widest border-b border-zinc-200/30 dark:border-zinc-800/30 pb-0.5 mb-0.5">
                              Recipes
                            </div>
                            {navFilteredRecipes.map((r) => {
                              const itemIdx = navFlatItems.findIndex(i => i.type === "recipe" && i.id === r.id);
                              const isSelected = navSelectedIndex === itemIdx;
                              return (
                                <button
                                  key={`nav-r-${r.id}`}
                                  onClick={() => navigateNavTo(navFlatItems[itemIdx].url)}
                                  onMouseEnter={() => setNavSelectedIndex(itemIdx)}
                                  className={`w-full px-2 py-1.5 text-left rounded-md flex flex-col gap-0.5 transition-all duration-100 text-[10px] ${
                                    isSelected 
                                      ? "bg-[#2563EB]/10 text-[#2563EB] dark:bg-[#2563EB]/25 dark:text-blue-400"
                                      : "text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200/40 dark:hover:bg-zinc-900/40"
                                  }`}
                                >
                                  <div className="flex items-center justify-between w-full">
                                    <div className="flex items-center gap-1.5 truncate font-semibold">
                                      <BookOpen className="w-3 h-3 opacity-60" />
                                      <span className="truncate">{r.id}</span>
                                    </div>
                                    {isSelected && <CornerDownRight className="w-2.5 h-2.5 opacity-80" />}
                                  </div>
                                </button>
                              );
                            })}
                          </div>
                        )}

                        {/* Users section */}
                        {navFilteredUsers.length > 0 && (
                          <div className="flex flex-col gap-0.5">
                            <div className="px-2 py-1 text-[8px] font-bold text-zinc-400 dark:text-zinc-550 uppercase tracking-widest border-b border-zinc-200/30 dark:border-zinc-800/30 pb-0.5 mb-0.5">
                              Users
                            </div>
                            {navFilteredUsers.map((u) => {
                              const itemIdx = navFlatItems.findIndex(i => i.type === "user" && i.id === u.username);
                              const isSelected = navSelectedIndex === itemIdx;
                              return (
                                <button
                                  key={`nav-u-${u.username}`}
                                  onClick={() => navigateNavTo(navFlatItems[itemIdx].url)}
                                  onMouseEnter={() => setNavSelectedIndex(itemIdx)}
                                  className={`w-full px-2 py-1.5 text-left rounded-md flex items-center justify-between transition-all duration-100 text-[10px] ${
                                    isSelected 
                                      ? "bg-[#2563EB]/10 text-[#2563EB] dark:bg-[#2563EB]/25 dark:text-blue-400 font-semibold"
                                      : "text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200/40 dark:hover:bg-zinc-900/40"
                                  }`}
                                >
                                  <div className="flex items-center gap-1.5 truncate">
                                    <User className="w-3 h-3 opacity-60" />
                                    <span className="truncate">@{u.username}</span>
                                  </div>
                                  {isSelected && <CornerDownRight className="w-2.5 h-2.5 opacity-80" />}
                                </button>
                              );
                            })}
                          </div>
                        )}
                      </>
                    )}
                  </div>
                </div>
              )}
            </div>

            {navItems.map((item, i) => {
              const commonClasses = "flex items-center flex-1 max-w-[120px] h-7 px-4 backdrop-blur-md border border-black/5 dark:border-white/5 bg-black/5 dark:bg-white/10 text-black/70 dark:text-white/80 rounded-md text-[10px] font-mono font-medium transition-all duration-300";
              
              return (
                <Link
                  key={`nav-item-${item.label}-${i}`}
                  href={item.href}
                  className={`${commonClasses} group hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black`}
                >
                  <span className="flex-1 text-left truncate">{item.label}</span>
                </Link>
              );
            })}
          </>
        )}
        {user ? (
          <div ref={dropdownRef} className="relative pointer-events-auto shrink-0">
            <button
              onClick={() => setIsDropdownOpen(!isDropdownOpen)}
              className="flex items-center gap-2 h-7 px-2 bg-black/5 dark:bg-white/5 border border-black/10 dark:border-white/10 hover:border-black/20 dark:hover:border-white/20 rounded-md shrink-0 cursor-pointer transition-all duration-150 select-none outline-none font-mono text-[10px] font-bold text-black dark:text-white uppercase tracking-wider"
            >
              {user.avatar_url ? (
                <Image src={user.avatar_url} alt="Avatar" width={16} height={16} className="w-4 h-4 rounded-full border border-black/10 dark:border-white/10 shrink-0" unoptimized />
              ) : (
                <div className="w-4 h-4 rounded-full bg-blue-500 text-white flex items-center justify-center text-[7px] font-bold shrink-0">
                  {user.username.substring(0, 2).toUpperCase()}
                </div>
              )}
              <span>{user.username}</span>
              <span className={`text-[6px] text-zinc-500 transition-transform duration-200 ${isDropdownOpen ? "rotate-180" : ""}`}>▼</span>
            </button>

            {isDropdownOpen && (
              <div className="absolute top-9 right-0 w-48 bg-[#f6f6f3]/95 dark:bg-[#171616]/95 border border-zinc-300 dark:border-zinc-800 text-black dark:text-white font-mono rounded-lg p-1 shadow-2xl backdrop-blur-xl z-50 select-none">
                {/* SVG Corner L-Brackets for premium brutalist look */}
                <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white pointer-events-none">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-y-[-1] pointer-events-none">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none">
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>

                <div className="px-2.5 py-1.5 text-[8px] uppercase tracking-wider text-zinc-400 dark:text-zinc-500 font-bold border-b border-black/5 dark:border-white/5">
                  Developer Actions
                </div>
                <Link
                  href="/registry/submit"
                  onClick={() => setIsDropdownOpen(false)}
                  className="flex items-center justify-between text-[10px] px-3 py-2 rounded-md hover:bg-blue-500/10 hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer transition-colors uppercase tracking-wider font-bold"
                >
                  Submit
                </Link>

                <div className="h-[1px] border-t border-dashed border-zinc-300 dark:border-zinc-800 my-1" />

                <Link
                  href={`/users/${user.username}`}
                  onClick={() => setIsDropdownOpen(false)}
                  className="flex items-center justify-between text-[10px] px-3 py-2 rounded-md hover:bg-zinc-200/50 dark:hover:bg-zinc-900/50 cursor-pointer transition-colors text-zinc-650 dark:text-zinc-350 hover:text-black dark:hover:text-white uppercase tracking-wider font-bold"
                >
                  Profile
                </Link>


                <div className="h-[1px] border-t border-dashed border-zinc-300 dark:border-zinc-800 my-1" />

                <button
                  onClick={() => {
                    setIsDropdownOpen(false);
                    logout();
                  }}
                  className="w-full text-left flex items-center justify-between text-[10px] px-3 py-2 rounded-md hover:bg-red-500/10 hover:text-red-650 dark:hover:text-red-400 cursor-pointer transition-colors font-bold uppercase tracking-wider"
                >
                  Sign out
                </button>
              </div>
            )}
          </div>
        ) : (
          <CTAButton label="get started" href="/login" variant="small" className="shrink-0 pointer-events-auto" />
        )}
      </div>

      <div className="max-w-7xl w-full mx-auto flex md:hidden flex-col gap-1 pointer-events-auto">
        <button 
          onClick={() => setIsMenuOpen(!isMenuOpen)}
          className={`flex items-center h-7 rounded-md transition-colors duration-300 w-full overflow-hidden ${isMenuOpen ? 'bg-black text-white dark:bg-white dark:text-black' : 'bg-[#2563EB] text-white'}`}
        >
          <span className="flex-1 font-mono text-[13px] font-medium leading-none tracking-tight text-left px-3">Bloc</span>
          <div className="flex items-center justify-center h-full aspect-square border-l border-white/10">
            <div className="grid grid-cols-3 gap-0.5">
              {[...Array(9)].map((_, i) => (
                <div key={i} className="w-0.5 h-0.5 bg-current rounded-full" />
              ))}
            </div>
          </div>
        </button>

        <div className={`flex flex-col gap-1 transition-all duration-300 overflow-hidden ${
          isMenuOpen 
            ? 'max-h-[500px] mt-1 p-2 bg-[#f6f6f3] dark:bg-[#171616] border border-zinc-300 dark:border-zinc-800 rounded-md shadow-lg' 
            : 'max-h-0 border-transparent'
        }`}>
          {isDocs ? (
            <>
              {/* Home Link */}
              <Link
                href="/"
                onClick={() => setIsMenuOpen(false)}
                className="flex items-center h-7 px-3 bg-zinc-100 dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200 hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black mb-1"
              >
                Home
              </Link>

              {/* Docs Search Trigger in Mobile Menu */}
              <button
                onClick={() => {
                  setOpenSearch(true);
                  setIsMenuOpen(false);
                }}
                className="flex items-center h-7 px-3 bg-zinc-100 dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200 text-left hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black cursor-pointer w-full border-none outline-none mb-1"
              >
                <span className="flex-1 text-left flex items-center">
                  <svg width="10" height="10" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className="mr-2 opacity-60">
                    <path d="M14.5 14.5L10.5 10.5M12.5 6.5C12.5 9.81371 9.81371 12.5 6.5 12.5C3.18629 12.5 0.5 9.81371 0.5 6.5C0.5 3.18629 3.18629 0.5 6.5 0.5C9.81371 0.5 12.5 3.18629 12.5 6.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                  <span>Search docs...</span>
                </span>
                <span className="text-[8px] opacity-50 font-bold bg-zinc-200 dark:bg-zinc-800 px-1 rounded select-none">⌘K</span>
              </button>

              {/* Submit Link */}
              <Link
                href="/registry/submit"
                onClick={() => setIsMenuOpen(false)}
                className="flex items-center h-7 px-3 bg-zinc-100 dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200 hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black"
              >
                Submit
              </Link>
              
              {/* Account/Login Actions */}
              {user ? (
                <div className="flex flex-col gap-1.5 w-full mt-2 border-t border-zinc-300 dark:border-zinc-800 pt-2 pointer-events-auto font-mono text-[10px]">
                  <div className="flex items-center gap-2 px-3 py-1 font-bold text-zinc-500 dark:text-zinc-450">
                    {user.avatar_url ? (
                      <Image src={user.avatar_url} alt="Avatar" width={16} height={16} className="w-4 h-4 rounded-full border border-zinc-300 dark:border-zinc-800" unoptimized />
                    ) : (
                      <div className="w-4 h-4 rounded-full bg-blue-500 text-white flex items-center justify-center text-[7px] font-bold">
                        {user.username.substring(0, 2).toUpperCase()}
                      </div>
                    )}
                    <span>@{user.username}</span>
                  </div>
                  
                  <button
                    onClick={() => {
                      logout();
                      setIsMenuOpen(false);
                    }}
                    className="flex items-center justify-center h-7 bg-red-605 hover:bg-red-700 text-white border border-red-700 rounded-md font-bold uppercase tracking-wider cursor-pointer animate-none"
                  >
                    Sign out
                  </button>
                </div>
              ) : (
                <CTAButton label="get started" href="/login" className="w-full mt-1 pointer-events-auto" variant="small" />
              )}
            </>
          ) : (
            <>
              {/* Search Trigger for non-docs mobile view */}
              <button
                onClick={() => {
                  setIsSearchOpen(true);
                  setIsMenuOpen(false);
                }}
                className="flex items-center h-7 px-3 bg-zinc-100 dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200 text-left hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black cursor-pointer w-full border-none outline-none mb-1"
              >
                <span className="flex-1 text-left flex items-center">
                  <svg width="10" height="10" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className="mr-2 opacity-60">
                    <path d="M14.5 14.5L10.5 10.5M12.5 6.5C12.5 9.81371 9.81371 12.5 6.5 12.5C3.18629 12.5 0.5 9.81371 0.5 6.5C0.5 3.18629 3.18629 0.5 6.5 0.5C9.81371 0.5 12.5 3.18629 12.5 6.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                  <span>Search registry...</span>
                </span>
              </button>

              {/* Home Link */}
              <Link
                href="/"
                onClick={() => setIsMenuOpen(false)}
                className="flex items-center h-7 px-3 bg-zinc-100 dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200 hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black mb-1"
              >
                Home
              </Link>

              {navItems.map((item, i) => {
                const commonClasses = "flex items-center h-7 px-3 bg-zinc-100 dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200";

                return (
                  <Link
                    key={`nav-item-mob-${item.label}-${i}`}
                    href={item.href}
                    onClick={() => setIsMenuOpen(false)}
                    className={`${commonClasses} hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black`}
                  >
                    <span className="flex-1 text-left">{item.label}</span>
                  </Link>
                );
              })}
              
              {user ? (
                <div className="flex flex-col gap-1.5 w-full mt-2 border-t border-zinc-300 dark:border-zinc-800 pt-2 pointer-events-auto font-mono text-[10px]">
                  <div className="flex items-center gap-2 px-3 py-1 font-bold text-zinc-500 dark:text-zinc-450 mb-1">
                    {user.avatar_url ? (
                      <Image src={user.avatar_url} alt="Avatar" width={16} height={16} className="w-4 h-4 rounded-full border border-zinc-350 dark:border-zinc-750" unoptimized />
                    ) : (
                      <div className="w-4 h-4 rounded-full bg-blue-500 text-white flex items-center justify-center text-[7px] font-bold">
                        {user.username.substring(0, 2).toUpperCase()}
                      </div>
                    )}
                    <span>@{user.username}</span>
                  </div>
                  
                  <Link 
                    href="/registry/submit"
                    onClick={() => setIsMenuOpen(false)}
                    className="flex items-center h-7 px-3 bg-[#2563EB] hover:bg-blue-650 border border-blue-600 text-white rounded-md font-bold uppercase tracking-wider justify-center"
                  >
                    Submit
                  </Link>

                  <Link 
                    href={`/users/${user.username}`}
                    onClick={() => setIsMenuOpen(false)}
                    className="flex items-center h-7 px-3 bg-zinc-200 dark:bg-zinc-800 hover:bg-zinc-300 dark:hover:bg-zinc-700 border border-zinc-300 dark:border-zinc-700 text-zinc-900 dark:text-zinc-150 rounded-md font-bold uppercase tracking-wider justify-center"
                  >
                    Profile
                  </Link>

                  <button
                    onClick={() => {
                      logout();
                      setIsMenuOpen(false);
                    }}
                    className="flex items-center justify-center h-7 bg-red-600 hover:bg-red-750 text-white border border-red-700 rounded-md font-bold uppercase tracking-wider cursor-pointer"
                  >
                    Sign out
                  </button>
                </div>
              ) : (
                <CTAButton label="get started" href="/login" className="w-full mt-1 pointer-events-auto" variant="small" />
              )}
            </>
          )}
        </div>
      </div>

      {/* Global Search Dialog Modal */}
      {isSearchOpen && (
        <div 
          className="fixed inset-0 bg-black/60 dark:bg-black/80 backdrop-blur-sm z-[9999] flex items-start justify-center pt-12 sm:pt-24 px-4 pointer-events-auto"
          onClick={() => setIsSearchOpen(false)}
        >
          <div 
            className="relative w-full max-w-lg bg-[#f6f6f3] dark:bg-[#121212] border border-zinc-300 dark:border-zinc-800 rounded-lg shadow-2xl flex flex-col overflow-hidden max-h-[85vh] sm:max-h-[520px]"
            onClick={(e) => e.stopPropagation()}
          >
            {/* L-Brackets for premium brutalist look */}
            <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white pointer-events-none">
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-y-[-1] pointer-events-none">
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none">
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>

            {/* Input container */}
            <div className="flex items-center gap-3 px-4 py-3.5 border-b border-zinc-200 dark:border-zinc-800">
              <Search className="w-4 h-4 text-zinc-400 shrink-0" />
              <input 
                autoFocus
                type="text"
                placeholder="Search models, recipes, users..."
                value={globalSearchQuery}
                onChange={(e) => setGlobalSearchQuery(e.target.value)}
                className="flex-grow bg-transparent border-none outline-none font-mono text-xs md:text-sm text-black dark:text-white placeholder-zinc-400"
              />
              <button 
                onClick={() => setIsSearchOpen(false)}
                className="shrink-0 font-mono text-[9px] bg-zinc-200/60 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400 px-1.5 py-0.5 rounded shadow-sm border border-zinc-300 dark:border-zinc-700 uppercase tracking-wider leading-none"
              >
                esc
              </button>
            </div>

            {/* Results box */}
            <div className="flex-grow overflow-y-auto p-2 flex flex-col gap-1 max-h-[380px]">
              {flatItems.length === 0 ? (
                <div className="py-12 text-center font-mono text-[11px] text-zinc-400">
                  No results found for "{globalSearchQuery}"
                </div>
              ) : (
                <>
                  {/* Models section */}
                  {filteredModels.length > 0 && (
                    <div className="flex flex-col gap-0.5 mb-2">
                      <div className="px-3 py-1.5 text-[9px] font-mono font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-widest border-b border-zinc-200/40 dark:border-zinc-800/40 pb-1 mb-1">
                        Models
                      </div>
                      {filteredModels.map((m, idx) => {
                        const itemIdx = flatItems.findIndex(i => i.type === "model" && i.id === m.title);
                        const isSelected = selectedIndex === itemIdx;
                        return (
                          <button
                            key={`m-${m.title}`}
                            onClick={() => navigateTo(flatItems[itemIdx].url)}
                            onMouseEnter={() => setSelectedIndex(itemIdx)}
                            className={`w-full px-3 py-2 text-left rounded-md flex items-center justify-between transition-all duration-100 font-mono text-xs ${
                              isSelected 
                                ? "bg-[#2563EB]/10 text-[#2563EB] dark:bg-[#2563EB]/20 dark:text-blue-400 font-medium"
                                : "text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200/40 dark:hover:bg-zinc-900/40"
                            }`}
                          >
                            <div className="flex items-center gap-2 truncate">
                              <Cpu className="w-3.5 h-3.5 opacity-60" />
                              <span className="truncate">{m.title}</span>
                            </div>
                            {isSelected && <CornerDownRight className="w-3 h-3 animate-pulse" />}
                          </button>
                        );
                      })}
                    </div>
                  )}

                  {/* Recipes section */}
                  {filteredRecipes.length > 0 && (
                    <div className="flex flex-col gap-0.5 mb-2">
                      <div className="px-3 py-1.5 text-[9px] font-mono font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-widest border-b border-zinc-200/40 dark:border-zinc-800/40 pb-1 mb-1">
                        Recipes
                      </div>
                      {filteredRecipes.map((r, idx) => {
                        const itemIdx = flatItems.findIndex(i => i.type === "recipe" && i.id === r.id);
                        const isSelected = selectedIndex === itemIdx;
                        return (
                          <button
                            key={`r-${r.id}`}
                            onClick={() => navigateTo(flatItems[itemIdx].url)}
                            onMouseEnter={() => setSelectedIndex(itemIdx)}
                            className={`w-full px-3 py-2.5 text-left rounded-md flex flex-col gap-0.5 transition-all duration-100 font-mono text-xs ${
                              isSelected 
                                ? "bg-[#2563EB]/10 text-[#2563EB] dark:bg-[#2563EB]/20 dark:text-blue-400"
                                : "text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200/40 dark:hover:bg-zinc-900/40"
                            }`}
                          >
                            <div className="flex items-center justify-between w-full">
                              <div className="flex items-center gap-2 truncate font-semibold">
                                <BookOpen className="w-3.5 h-3.5 opacity-60" />
                                <span className="truncate">{r.id}</span>
                              </div>
                              {isSelected && <CornerDownRight className="w-3 h-3 animate-pulse" />}
                            </div>
                            {r.description && (
                              <span className="text-[10px] text-zinc-400 pl-5 line-clamp-1 truncate max-w-[90%]">
                                {r.description}
                              </span>
                            )}
                          </button>
                        );
                      })}
                    </div>
                  )}

                  {/* Users section */}
                  {filteredUsers.length > 0 && (
                    <div className="flex flex-col gap-0.5">
                      <div className="px-3 py-1.5 text-[9px] font-mono font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-widest border-b border-zinc-200/40 dark:border-zinc-800/40 pb-1 mb-1">
                        Users
                      </div>
                      {filteredUsers.map((u, idx) => {
                        const itemIdx = flatItems.findIndex(i => i.type === "user" && i.id === u.username);
                        const isSelected = selectedIndex === itemIdx;
                        return (
                          <button
                            key={`u-${u.username}`}
                            onClick={() => navigateTo(flatItems[itemIdx].url)}
                            onMouseEnter={() => setSelectedIndex(itemIdx)}
                            className={`w-full px-3 py-2 text-left rounded-md flex items-center justify-between transition-all duration-100 font-mono text-xs ${
                              isSelected 
                                ? "bg-[#2563EB]/10 text-[#2563EB] dark:bg-[#2563EB]/20 dark:text-blue-400 font-medium"
                                : "text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200/40 dark:hover:bg-zinc-900/40"
                            }`}
                          >
                            <div className="flex items-center gap-2 truncate">
                              <User className="w-3.5 h-3.5 opacity-60" />
                              <span className="truncate">@{u.username}</span>
                              {u.display_name && u.display_name !== u.username && (
                                <span className="text-[10px] text-zinc-400 truncate">({u.display_name})</span>
                              )}
                            </div>
                            {isSelected && <CornerDownRight className="w-3 h-3 animate-pulse" />}
                          </button>
                        );
                      })}
                    </div>
                  )}
                </>
              )}
            </div>

            {/* Footer status info */}
            <div className="px-4 py-2 bg-zinc-100 dark:bg-zinc-900 border-t border-zinc-200 dark:border-zinc-800 flex items-center justify-between text-[8px] font-mono text-zinc-400 dark:text-zinc-500 select-none uppercase tracking-wider">
              <div className="flex items-center gap-3">
                <span>↑↓ Navigate</span>
                <span>↵ Select</span>
              </div>
              <div className="flex items-center gap-1.5">
                <Command className="w-2.5 h-2.5" />
                <span>K to close</span>
              </div>
            </div>

          </div>
        </div>
      )}
    </nav>
  );
}
