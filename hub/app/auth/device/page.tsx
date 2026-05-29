"use client";

import React, { useState } from "react";
import { useAuth } from "@/context/AuthContext";
import { supabase } from "@/lib/supabase";
import { Loader2, ArrowRight, Check } from "lucide-react";

type PageState = "idle" | "authorizing" | "success" | "error";

export default function DeviceAuthPage() {
  const { user, loading } = useAuth();
  const [code, setCode] = useState("");
  const [state, setState] = useState<PageState>("idle");
  const [errorMsg, setErrorMsg] = useState("");
  const [authorizedAs, setAuthorizedAs] = useState("");

  // Auto-format input as XXXX-XXXX as the user types
  function handleCodeChange(e: React.ChangeEvent<HTMLInputElement>) {
    let val = e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, "");
    if (val.length > 4) val = val.slice(0, 4) + "-" + val.slice(4, 8);
    setCode(val);
  }

  async function handleSignIn() {
    if (!supabase) return;
    // Redirect back to this page after GitHub OAuth
    await supabase.auth.signInWithOAuth({
      provider: "github",
      options: { redirectTo: `${window.location.origin}/auth/device` },
    });
  }

  async function handleAuthorize() {
    if (!code.match(/^[A-Z0-9]{4}-[A-Z0-9]{4}$/)) {
      setErrorMsg("Enter a valid 8-character code (e.g. ABCD-1234)");
      setState("error");
      return;
    }
    if (!supabase) return;

    setState("authorizing");
    setErrorMsg("");

    // Get the user's current Supabase session token
    const {
      data: { session },
    } = await supabase.auth.getSession();

    if (!session?.access_token) {
      setErrorMsg("Your session expired — please sign in again.");
      setState("error");
      return;
    }

    const res = await fetch("/api/auth/device/authorize", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${session.access_token}`,
      },
      body: JSON.stringify({ user_code: code }),
    });

    const data = await res.json();

    if (!res.ok) {
      setErrorMsg(data.error || "Authorization failed — please try again.");
      setState("error");
      return;
    }

    setAuthorizedAs(data.username);
    setState("success");
  }

  return (
    <div className="flex-grow flex flex-col items-center justify-center p-6 min-h-[80vh] relative select-none">
      
      {/* Device Auth Box */}
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

        {/* ── Loading State ── */}
        {loading && (
          <div className="flex flex-col items-center justify-center py-12">
            <Loader2 className="w-6 h-6 animate-spin text-zinc-500" />
            <p className="text-zinc-500 font-mono text-[9px] uppercase tracking-wider mt-4">
              Loading session...
            </p>
          </div>
        )}

        {/* ── Success State ── */}
        {!loading && state === "success" && (
          <div className="flex flex-col items-center gap-6 text-center">
            <div className="w-12 h-12 rounded-full border border-emerald-500/30 bg-emerald-500/10 flex items-center justify-center">
              <Check className="w-5 h-5 text-emerald-600 dark:text-emerald-400" />
            </div>
            <div>
              <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-emerald-500/20 bg-emerald-500/5 text-emerald-650 dark:text-emerald-400 font-bold">
                Success
              </span>
              <h1 className="text-3xl font-semibold tracking-tight font-switzer text-black dark:text-white mt-4 leading-none animate-fade-in">
                CLI Authorized
              </h1>
              <p className="text-zinc-550 dark:text-zinc-400 font-switzer font-medium text-xs leading-relaxed mt-3.5 max-w-sm mx-auto">
                Signed in as <span className="text-black dark:text-white font-bold">@{authorizedAs}</span>. You can now close this tab — your terminal is ready.
              </p>
            </div>

            <div className="w-full border border-zinc-300 dark:border-zinc-800 bg-zinc-200/40 dark:bg-zinc-900/40 px-4 py-3 text-left font-mono text-[11px] text-zinc-600 dark:text-zinc-400 leading-normal">
              $ bloc login<br />
              <span className="text-emerald-600 dark:text-emerald-400 font-semibold">✓ Logged in as {authorizedAs}</span>
            </div>
          </div>
        )}

        {/* ── Not Signed In State ── */}
        {!loading && state !== "success" && !user && (
          <div className="flex flex-col gap-6">
            <div className="text-center">
              <h1 className="text-3xl font-semibold tracking-tight font-switzer text-black dark:text-white leading-none">
                Authorize Bloc CLI
              </h1>
              <p className="text-zinc-550 dark:text-zinc-400 font-switzer font-medium text-xs leading-relaxed mt-3.5 max-w-sm mx-auto">
                Sign in with GitHub first, then enter the code shown in your terminal to authorize the CLI.
              </p>
            </div>

            <button
              onClick={handleSignIn}
              className="group relative w-full h-11 border border-zinc-850 dark:border-zinc-100 bg-black text-white dark:bg-white dark:text-black hover:bg-zinc-800 dark:hover:bg-zinc-100 font-mono text-[11px] uppercase font-bold tracking-wider cursor-pointer transition-all rounded-none flex items-center justify-center gap-3"
            >
              {/* Tactile SVGs */}
              <svg viewBox="0 0 12 12" className="absolute top-0 left-0 w-2 h-2 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute top-0 right-0 w-2 h-2 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute bottom-0 left-0 w-2 h-2 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute bottom-0 right-0 w-2 h-2 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>

              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
                <path d="M9 18c-4.51 2-5-2-7-2" />
              </svg>
              Sign In with GitHub
            </button>

            <div className="mt-2 border-t border-zinc-200 dark:border-zinc-800 pt-6 text-center font-mono text-[9px] text-zinc-400 leading-normal">
              Secure OAuth connection handles authentication. <br />
              We never store your GitHub passwords or read private repos.
            </div>
          </div>
        )}

        {/* ── Signed In — Enter Code State ── */}
        {!loading && state !== "success" && user && (
          <div className="flex flex-col gap-6">
            <div className="text-center">
              <h1 className="text-3xl font-semibold tracking-tight font-switzer text-black dark:text-white leading-none">
                Authorize Bloc CLI
              </h1>
              <p className="text-zinc-550 dark:text-zinc-400 font-switzer font-medium text-xs leading-relaxed mt-3.5 max-w-sm mx-auto">
                Enter the code shown in your terminal to grant the CLI access to your account.
              </p>
            </div>

            {/* User Badge */}
            <div className="relative w-full h-10 border border-zinc-300 dark:border-zinc-800 bg-zinc-200/40 dark:bg-zinc-900/40 px-3 flex items-center justify-between font-mono text-[11px] text-zinc-650 dark:text-zinc-400">
              <div className="flex items-center gap-2">
                <img
                  src={user.avatar_url}
                  alt={user.username}
                  className="w-5 h-5 rounded-full"
                />
                <span>@{user.username}</span>
              </div>
              <div className="font-mono text-[8px] uppercase tracking-wider text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 font-bold">
                Signed In
              </div>
            </div>

            {/* Code input */}
            <div className="flex flex-col gap-2.5">
              <label className="block font-mono text-[9px] uppercase font-bold text-zinc-500">
                Terminal Code
              </label>
              <input
                type="text"
                value={code}
                onChange={handleCodeChange}
                maxLength={9}
                placeholder="ABCD-1234"
                disabled={state === "authorizing"}
                className="w-full h-12 px-3 border border-zinc-300 dark:border-zinc-800 bg-transparent text-center text-xl font-mono tracking-[0.3em] placeholder:text-zinc-300 dark:placeholder:text-zinc-700 outline-none text-black dark:text-white rounded-none disabled:opacity-50 focus:border-blue-500 transition-colors uppercase"
                onKeyDown={(e) => e.key === "Enter" && handleAuthorize()}
              />
            </div>

            {/* Error Message */}
            {state === "error" && errorMsg && (
              <div className="bg-red-500/10 border border-red-500/20 px-4 py-3 rounded-none">
                <p className="text-red-650 dark:text-red-400 text-xs font-switzer font-medium leading-normal">{errorMsg}</p>
              </div>
            )}

            {/* Authorize Button */}
            <button
              onClick={handleAuthorize}
              disabled={state === "authorizing" || code.length < 9}
              className="group relative w-full h-11 border border-zinc-850 dark:border-zinc-100 bg-black text-white dark:bg-white dark:text-black hover:bg-zinc-800 dark:hover:bg-zinc-100 font-mono text-[11px] uppercase font-bold tracking-wider cursor-pointer transition-all rounded-none flex items-center justify-center gap-3 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {/* Tactile SVGs */}
              <svg viewBox="0 0 12 12" className="absolute top-0 left-0 w-2 h-2 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute top-0 right-0 w-2 h-2 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute bottom-0 left-0 w-2 h-2 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg viewBox="0 0 12 12" className="absolute bottom-0 right-0 w-2 h-2 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5">
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>

              {state === "authorizing" ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Authorizing...
                </>
              ) : (
                <>
                  Authorize CLI Access
                  <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-0.5" />
                </>
              )}
            </button>

            <p className="text-zinc-400 font-mono text-[9px] text-center leading-normal">
              This grants the CLI read/write access to your Bloc account. <br />
              Run <span className="font-semibold text-zinc-550 dark:text-zinc-350">bloc logout</span> at any time to revoke access.
            </p>
          </div>
        )}

      </div>
    </div>
  );
}
