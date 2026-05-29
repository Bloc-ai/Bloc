"use client";

import React, { useState } from "react";
import { useAuth } from "@/context/AuthContext";
import { supabase } from "@/lib/supabase";

type PageState = "idle" | "authorizing" | "success" | "error";

export default function DeviceAuthPage() {
  const { user, loading, login } = useAuth();
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
    <div className="min-h-screen bg-zinc-950 flex flex-col items-center justify-center px-4">
      {/* Background glow */}
      <div
        className="pointer-events-none fixed inset-0 z-0"
        style={{
          background:
            "radial-gradient(ellipse 60% 40% at 50% 0%, rgba(99,102,241,0.12) 0%, transparent 70%)",
        }}
      />

      <div className="relative z-10 w-full max-w-md">
        {/* Logo */}
        <div className="flex items-center gap-3 mb-10 justify-center">
          <div className="w-8 h-8 rounded-lg bg-indigo-500 flex items-center justify-center">
            <svg viewBox="0 0 24 24" fill="white" className="w-5 h-5">
              <path d="M13 3L4 14h8l-1 7 9-11h-8l1-10z" />
            </svg>
          </div>
          <span className="text-white font-semibold text-xl tracking-tight">
            Bloc
          </span>
        </div>

        <div className="bg-zinc-900 border border-zinc-800 rounded-2xl p-8 shadow-2xl">
          {/* ── Loading ── */}
          {loading && (
            <div className="flex flex-col items-center gap-4 py-6">
              <div className="w-6 h-6 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" />
              <p className="text-zinc-500 text-sm">Loading...</p>
            </div>
          )}

          {/* ── Success ── */}
          {!loading && state === "success" && (
            <div className="flex flex-col items-center gap-5 py-4 text-center">
              <div className="w-14 h-14 rounded-full bg-emerald-500/15 flex items-center justify-center">
                <svg
                  className="w-7 h-7 text-emerald-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </div>
              <div>
                <h1 className="text-white text-xl font-semibold mb-1">
                  CLI authorized
                </h1>
                <p className="text-zinc-400 text-sm">
                  Signed in as{" "}
                  <span className="text-white font-medium">@{authorizedAs}</span>
                  . You can close this tab — your terminal is ready.
                </p>
              </div>
              <div className="w-full bg-zinc-800/60 rounded-xl px-4 py-3 text-left">
                <p className="text-zinc-500 text-xs font-mono">
                  $ bloc login<br />
                  <span className="text-emerald-400">✓ Logged in as {authorizedAs}</span>
                </p>
              </div>
            </div>
          )}

          {/* ── Not signed in ── */}
          {!loading && state !== "success" && !user && (
            <div className="flex flex-col gap-6">
              <div className="text-center">
                <h1 className="text-white text-xl font-semibold mb-2">
                  Authorize Bloc CLI
                </h1>
                <p className="text-zinc-400 text-sm leading-relaxed">
                  Sign in with GitHub first, then enter the code shown in your
                  terminal to authorize the CLI.
                </p>
              </div>

              <button
                onClick={handleSignIn}
                className="flex items-center justify-center gap-3 w-full bg-white hover:bg-zinc-100 text-zinc-900 font-medium py-2.5 px-4 rounded-xl transition-colors text-sm"
              >
                <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                </svg>
                Sign in with GitHub
              </button>
            </div>
          )}

          {/* ── Signed in — show code entry ── */}
          {!loading && state !== "success" && user && (
            <div className="flex flex-col gap-6">
              <div className="text-center">
                <h1 className="text-white text-xl font-semibold mb-2">
                  Authorize Bloc CLI
                </h1>
                <p className="text-zinc-400 text-sm leading-relaxed">
                  Enter the code shown in your terminal to grant the CLI access
                  to your account.
                </p>
              </div>

              {/* Signed-in badge */}
              <div className="flex items-center gap-2 bg-zinc-800/60 rounded-xl px-4 py-2.5">
                <img
                  src={`https://github.com/${user.username}.png?size=32`}
                  alt={user.username}
                  className="w-6 h-6 rounded-full"
                />
                <span className="text-zinc-300 text-sm">
                  Signed in as{" "}
                  <span className="text-white font-medium">@{user.username}</span>
                </span>
              </div>

              {/* Code input */}
              <div className="flex flex-col gap-2">
                <label className="text-zinc-400 text-xs font-medium uppercase tracking-wider">
                  Terminal Code
                </label>
                <input
                  type="text"
                  value={code}
                  onChange={handleCodeChange}
                  maxLength={9}
                  placeholder="ABCD-1234"
                  disabled={state === "authorizing"}
                  className="w-full bg-zinc-800 border border-zinc-700 rounded-xl px-4 py-3 text-white text-center text-2xl font-mono tracking-[0.3em] placeholder:text-zinc-600 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors disabled:opacity-50"
                  onKeyDown={(e) => e.key === "Enter" && handleAuthorize()}
                />
              </div>

              {/* Error message */}
              {state === "error" && errorMsg && (
                <div className="flex items-start gap-2.5 bg-red-500/10 border border-red-500/20 rounded-xl px-4 py-3">
                  <svg
                    className="w-4 h-4 text-red-400 mt-0.5 shrink-0"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <circle cx="12" cy="12" r="10" />
                    <line x1="12" y1="8" x2="12" y2="12" />
                    <line x1="12" y1="16" x2="12.01" y2="16" />
                  </svg>
                  <p className="text-red-400 text-sm">{errorMsg}</p>
                </div>
              )}

              {/* Authorize button */}
              <button
                onClick={handleAuthorize}
                disabled={state === "authorizing" || code.length < 9}
                className="flex items-center justify-center gap-2 w-full bg-indigo-600 hover:bg-indigo-500 disabled:bg-zinc-700 disabled:text-zinc-500 text-white font-medium py-3 px-4 rounded-xl transition-colors text-sm"
              >
                {state === "authorizing" ? (
                  <>
                    <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                    Authorizing...
                  </>
                ) : (
                  "Authorize CLI Access"
                )}
              </button>

              <p className="text-zinc-600 text-xs text-center">
                This grants the CLI read/write access to your Bloc account.
                <br />
                Run{" "}
                <span className="font-mono text-zinc-500">bloc logout</span> at
                any time to revoke access.
              </p>
            </div>
          )}
        </div>

        <p className="text-center text-zinc-700 text-xs mt-6">
          Bloc Hub · OAuth Device Authorization
        </p>
      </div>
    </div>
  );
}
