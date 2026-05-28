"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import Link from "next/link";
import Image from "next/image";
import { usePathname } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { useSearchContext } from "fumadocs-ui/contexts/search";

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
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const { user, logout } = useAuth();
  
  const pathname = usePathname();
  const isDocs = pathname?.startsWith("/docs");
  const { setOpenSearch } = useSearchContext();
  
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsDropdownOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

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
          { label: "Docs", href: "/docs" },
          { label: "Blog", href: "/blog" },
        ];
  }, [user]);

  if (isDocs) {
    return (
      <nav className="fixed top-0 left-0 right-0 h-12 z-50 bg-[#f6f6f3]/95 dark:bg-[#171616]/95 border-b border-zinc-200 dark:border-zinc-800 backdrop-blur-md pointer-events-auto select-none flex items-center px-4 md:px-6 justify-between gap-4">
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
    <nav className="fixed top-0 left-0 right-0 z-50 flex flex-col px-4 pt-2 gap-1 pointer-events-none">
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
            {/* Interactive Search Bar */}
            <div className="flex-1 relative h-7">
              <div className="flex items-center h-full px-3 bg-black/5 dark:bg-white/5 border border-black/10 dark:border-white/10 rounded-md gap-2 transition-all duration-300 hover:border-black/20 dark:hover:border-white/20 focus-within:border-blue-500 group/search">
                <svg width="12" height="12" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className="text-black/40 dark:text-white/40 group-hover/search:text-black/60 dark:group-hover/search:text-white/60">
                  <path d="M14.5 14.5L10.5 10.5M12.5 6.5C12.5 9.81371 9.81371 12.5 6.5 12.5C3.18629 12.5 0.5 9.81371 0.5 6.5C0.5 3.18629 3.18629 0.5 6.5 0.5C9.81371 0.5 12.5 3.18629 12.5 6.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search models, recipes, users..."
                  className="w-full bg-transparent border-none outline-none font-mono text-[10px] text-black dark:text-white placeholder-black/40 dark:placeholder-white/40 leading-none h-full"
                />
              </div>
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

      <div className="flex md:hidden flex-col gap-1 w-full pointer-events-auto">
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
        <div className={`flex flex-col gap-1 transition-all duration-300 overflow-hidden ${isMenuOpen ? 'max-h-[400px] mt-0' : 'max-h-0'}`}>
          {isDocs ? (
            <>
              {/* Docs Search Trigger in Mobile Menu */}
              <button
                onClick={() => {
                  setOpenSearch(true);
                  setIsMenuOpen(false);
                }}
                className="flex items-center h-7 px-3 bg-black/5 dark:bg-white/10 backdrop-blur-md border border-black/5 dark:border-white/5 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200 text-left hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black cursor-pointer w-full border-none outline-none"
              >
                <span className="flex-1 text-left flex items-center">
                  <svg width="10" height="10" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className="mr-2 opacity-60">
                    <path d="M14.5 14.5L10.5 10.5M12.5 6.5C12.5 9.81371 9.81371 12.5 6.5 12.5C3.18629 12.5 0.5 9.81371 0.5 6.5C0.5 3.18629 3.18629 0.5 6.5 0.5C9.81371 0.5 12.5 3.18629 12.5 6.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                  <span>Search docs...</span>
                </span>
                <span className="text-[8px] opacity-50 font-bold bg-black/10 dark:bg-white/10 px-1 rounded select-none">⌘K</span>
              </button>

              {/* Submit Link */}
              <Link
                href="/registry/submit"
                onClick={() => setIsMenuOpen(false)}
                className="flex items-center h-7 px-3 bg-black/5 dark:bg-white/10 backdrop-blur-md border border-black/5 dark:border-white/5 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200 hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black"
              >
                Submit
              </Link>
              
              {/* Account/Login Actions */}
              {user ? (
                <div className="flex flex-col gap-1 w-full mt-2 border-t border-black/5 dark:border-white/5 pt-2 pointer-events-auto font-mono text-[10px]">
                  <div className="flex items-center gap-2 px-3 py-1 font-bold text-zinc-450 dark:text-zinc-550">
                    {user.avatar_url ? (
                      <Image src={user.avatar_url} alt="Avatar" width={16} height={16} className="w-4 h-4 rounded-full border border-black/10 dark:border-white/10" unoptimized />
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
                    className="flex items-center justify-center h-7 bg-red-500/10 hover:bg-red-500/20 border border-red-500/20 text-red-650 dark:text-red-400 rounded-md font-bold uppercase tracking-wider cursor-pointer"
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
              {navItems.map((item, i) => {
                const commonClasses = "flex items-center h-7 px-3 bg-black/5 dark:bg-white/10 backdrop-blur-md border border-black/5 dark:border-white/5 rounded-md text-[10px] font-mono font-medium text-black/70 dark:text-white/80 transition-all duration-200";

                return (
                  <Link
                    key={`nav-item-mob-${item.label}-${i}`}
                    href={item.href}
                    className={`${commonClasses} hover:bg-black hover:text-white dark:hover:bg-white dark:hover:text-black`}
                  >
                    <span className="flex-1 text-left">{item.label}</span>
                  </Link>
                );
              })}
              {user ? (
                <div className="flex flex-col gap-1.5 w-full mt-2 border-t border-black/5 dark:border-white/5 pt-2 pointer-events-auto font-mono text-[10px]">
                  <div className="flex items-center gap-2 px-3 py-1 font-bold text-zinc-400">
                    {user.avatar_url ? (
                      <Image src={user.avatar_url} alt="Avatar" width={16} height={16} className="w-4 h-4 rounded-full border border-black/10 dark:border-white/10" unoptimized />
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
                    className="flex items-center h-7 px-3 bg-blue-500/10 hover:bg-blue-500/20 border border-blue-500/20 text-blue-600 dark:text-blue-400 rounded-md font-bold uppercase tracking-wider"
                  >
                    Submit
                  </Link>

                  <Link 
                    href={`/users/${user.username}`}
                    onClick={() => setIsMenuOpen(false)}
                    className="flex items-center h-7 px-3 bg-black/5 dark:bg-white/5 hover:bg-black/10 dark:hover:bg-white/10 border border-black/5 dark:border-white/5 text-zinc-650 dark:text-zinc-350 hover:text-black dark:hover:text-white rounded-md font-bold uppercase tracking-wider"
                  >
                    Profile
                  </Link>


                  <button
                    onClick={() => {
                      logout();
                      setIsMenuOpen(false);
                    }}
                    className="flex items-center justify-center h-7 bg-red-500/10 hover:bg-red-500/20 border border-red-500/20 text-red-650 dark:text-red-400 rounded-md font-bold uppercase tracking-wider cursor-pointer"
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
    </nav>
  );
}
