"use client";

import { useState, useEffect } from "react";
import { X } from "lucide-react";

export default function Banner() {
  const [isVisible, setIsVisible] = useState(false);
  const [isRendered, setIsRendered] = useState(false);

  useEffect(() => {
    const dismissed = localStorage.getItem("bloc-engine-bug-banner-dismissed");
    if (dismissed !== "true") {
      // Defer state updates to avoid synchronous setState inside useEffect warning
      const timer = setTimeout(() => {
        setIsRendered(true);
        setIsVisible(true);
        document.documentElement.style.setProperty("--banner-height", "26px");
      }, 50);
      return () => clearTimeout(timer);
    }
  }, []);

  const handleDismiss = () => {
    setIsVisible(false);
    document.documentElement.style.setProperty("--banner-height", "0px");
    localStorage.setItem("bloc-engine-bug-banner-dismissed", "true");
    
    // Unmount after transition finishes
    setTimeout(() => {
      setIsRendered(false);
    }, 300);
  };

  if (!isRendered) return null;

  return (
    <div
      className={`fixed top-0 left-0 right-0 z-[100] flex items-center justify-center bg-[#2563EB] text-white px-4 transition-all duration-300 ease-in-out overflow-hidden font-mono select-none ${
        isVisible ? "h-[26px] opacity-100" : "h-0 opacity-0"
      }`}
    >
      <div className="flex items-center justify-center w-full max-w-7xl mx-auto relative text-center">
        <span className="text-[11px] font-medium leading-none tracking-tight pr-6">
          <span className="hidden sm:inline">Notice: The new execution engine has some bugs. We&apos;re actively working to fix everything. Thanks for your patience!</span>
          <span className="inline sm:hidden">The new execution engine has bugs. We&apos;re actively fixing them.</span>
        </span>
        <button
          onClick={handleDismiss}
          className="absolute right-0 top-1/2 -translate-y-1/2 p-1 hover:bg-white/10 rounded transition-colors cursor-pointer"
          aria-label="Close announcement"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  );
}
