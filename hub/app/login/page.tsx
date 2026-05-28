"use client";

import React, { useState, useEffect, Suspense } from "react";
import { useAuth } from "@/context/AuthContext";
import { useSearchParams, useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";

export default function LoginPage() {
  return (
    <Suspense fallback={
      <div className="flex-grow flex items-center justify-center min-h-[80vh]">
        <Loader2 className="w-6 h-6 animate-spin text-zinc-500" />
      </div>
    }>
      <LoginContent />
    </Suspense>
  );
}

function LoginContent() {
  const { user, loading, login } = useAuth();
  const searchParams = useSearchParams();
  const router = useRouter();
  const [isLoggingIn, setIsLoggingIn] = useState(false);

  useEffect(() => {
    if (!loading && user) {
      const next = searchParams.get("next") || "/registry";
      router.replace(next);
    }
  }, [user, loading, searchParams, router]);

  const handleLogin = async () => {
    setIsLoggingIn(true);
    try {
      await login();
    } catch (e) {
      setIsLoggingIn(false);
    }
  };

  return (
    <div className="flex-grow flex flex-col items-center justify-center p-6 min-h-[80vh] relative select-none">
      
      {/* Login Box */}
      <div className="relative w-full max-w-md border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-8 md:p-10 rounded-none shadow-xl">
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
          <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 bg-zinc-200/50 dark:bg-zinc-900/50 text-zinc-500 dark:text-zinc-400">
            Secure Registry Access
          </span>
          <h1 className="text-3xl font-semibold tracking-tight font-switzer text-black dark:text-white mt-4 leading-tight">
            Sign in to Bloc Hub
          </h1>
          <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-xs leading-relaxed mt-2.5 max-w-sm mx-auto">
            Discover, upload, and deploy low-latency local AI model recipes using high-performance hardware constraints.
          </p>
        </div>

        {/* Dynamic Auth Button */}
        <button
          onClick={handleLogin}
          disabled={isLoggingIn}
          className="group relative w-full h-11 border border-zinc-850 dark:border-zinc-100 bg-black text-white dark:bg-white dark:text-black hover:bg-zinc-800 dark:hover:bg-zinc-100 font-mono text-[11px] uppercase font-bold tracking-wider cursor-pointer select-none transition-all rounded-none flex items-center justify-center gap-3 disabled:opacity-75 disabled:cursor-not-allowed"
        >
          {/* Tactile SVGs */}
          <svg 
            viewBox="0 0 12 12" 
            className="absolute top-0 left-0 w-2 h-2 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5"
          >
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg 
            viewBox="0 0 12 12" 
            className="absolute top-0 right-0 w-2 h-2 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5"
          >
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg 
            viewBox="0 0 12 12" 
            className="absolute bottom-0 left-0 w-2 h-2 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5"
          >
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg 
            viewBox="0 0 12 12" 
            className="absolute bottom-0 right-0 w-2 h-2 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5"
          >
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>

          {isLoggingIn ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              Redirecting...
            </>
          ) : (
            <>
              <svg 
                className="w-4 h-4" 
                viewBox="0 0 24 24" 
                fill="none" 
                stroke="currentColor" 
                strokeWidth="2.5" 
                strokeLinecap="round" 
                strokeLinejoin="round"
              >
                <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
                <path d="M9 18c-4.51 2-5-2-7-2" />
              </svg>
              Sign In with GitHub
            </>
          )}
        </button>

        {/* Footer info */}
        <div className="mt-8 border-t border-zinc-200 dark:border-zinc-800 pt-6 text-center select-none font-mono text-[9px] text-zinc-400">
          Secure OAuth connection handles authentication. <br />
          We never store your GitHub passwords or read private repos.
        </div>
      </div>
    </div>
  );
}
