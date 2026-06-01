"use client";

import React, { useState } from "react";
import Link from "next/link";
import ShortcutButton from "@/components/ShortcutButton";

type OS = "macos" | "linux" | "windows";

export default function InstallationClient() {
  const [activeOS, setActiveOS] = useState<OS>("macos");
  const [copiedText, setCopiedText] = useState<string | null>(null);

  const installCommands: Record<OS, { label: string; cmd: string }[]> = {
    macos: [
      {
        label: "Via Homebrew (Recommended)",
        cmd: "brew tap bloc-ai/bloc && brew install bloc"
      },
      {
        label: "Via Manual Tarball (Apple Silicon / M-Series)",
        cmd: "curl -L https://github.com/Bloc-ai/Bloc/releases/download/v0.2.1/bloc_darwin_arm64.tar.gz | tar -xz && sudo mv bloc /usr/local/bin/"
      },
      {
        label: "Via Manual Tarball (Intel Core)",
        cmd: "curl -L https://github.com/Bloc-ai/Bloc/releases/download/v0.2.1/bloc_darwin_amd64.tar.gz | tar -xz && sudo mv bloc /usr/local/bin/"
      }
    ],
    linux: [
      {
        label: "Debian / Ubuntu (AMD64 .deb)",
        cmd: "wget https://github.com/Bloc-ai/Bloc/releases/download/v0.2.1/bloc_0.2.1_linux_amd64.deb && sudo dpkg -i bloc_0.2.1_linux_amd64.deb"
      },
      {
        label: "RedHat / Fedora / CentOS (AMD64 .rpm)",
        cmd: "sudo dnf install https://github.com/Bloc-ai/Bloc/releases/download/v0.2.1/bloc_0.2.1_linux_amd64.rpm"
      },
      {
        label: "Via Manual Tarball (AMD64)",
        cmd: "curl -L https://github.com/Bloc-ai/Bloc/releases/download/v0.2.1/bloc_linux_amd64.tar.gz | tar -xz && sudo mv bloc /usr/local/bin/"
      }
    ],
    windows: [
      {
        label: "Windows Installer (Direct MSI Download)",
        cmd: "https://github.com/Bloc-ai/Bloc/releases/download/v0.2.1/bloc_windows_amd64.msi"
      },
      {
        label: "Via PowerShell (Manual Zip Setup)",
        cmd: "Invoke-WebRequest -Uri \"https://github.com/Bloc-ai/Bloc/releases/download/v0.2.1/bloc_windows_amd64.zip\" -OutFile \"bloc.zip\"; Expand-Archive \"bloc.zip\" -DestinationPath \"$env:USERPROFILE\\bin\"; [Environment]::SetEnvironmentVariable(\"Path\", $env:Path + \";$env:USERPROFILE\\bin\", \"User\")"
      }
    ]
  };

  const handleCopy = (text: string) => {
    if (text.startsWith("http")) {
      window.open(text, "_blank");
      return;
    }
    navigator.clipboard.writeText(text);
    setCopiedText(text);
    setTimeout(() => setCopiedText(null), 1500);
  };

  return (
    <div className="max-w-4xl w-full mx-auto px-6 py-16 pt-24">
      {/* Back link */}
      <Link href="/registry" className="inline-flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors mb-12">
        <svg width="8" height="8" viewBox="0 0 10 10" fill="none" xmlns="http://www.w3.org/2000/svg" className="transform rotate-180">
          <path d="M1 9L9 1M9 1H1M9 1V9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
        Back to registry
      </Link>

      {/* Hero Header */}
      <div className="w-full text-left mb-12">
        <h1 className="text-3xl md:text-4xl font-semibold tracking-tight font-switzer text-black dark:text-white mb-4 leading-tight">
          Installation & Setup
        </h1>
        <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-3xl leading-relaxed">
          Install the Bloc CLI to manage resources, scan local GPU bounds, and deploy optimized model configurations locally in one command.
        </p>
      </div>

      {/* OS Tab Selector Wrapper Box */}
      <div className="relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-8 rounded-none mb-12">
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

        {/* Operating System Tab Controls */}
        <div className="flex gap-2 border-b border-zinc-200 dark:border-zinc-850 pb-4 mb-6">
          {(["macos", "linux", "windows"] as OS[]).map((os) => (
            <button
              key={os}
              onClick={() => setActiveOS(os)}
              className={`px-3 py-1 font-mono text-[10px] uppercase tracking-wider border transition-all cursor-pointer ${
                activeOS === os
                  ? "bg-zinc-900 border-zinc-900 text-white dark:bg-white dark:border-white dark:text-black font-bold"
                  : "bg-zinc-200/50 border-zinc-300 text-zinc-650 hover:bg-zinc-200 dark:bg-zinc-900/30 dark:border-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-900"
              }`}
            >
              {os === "macos" ? "macOS" : os}
            </button>
          ))}
        </div>

        {/* Installation Instructions for Active OS */}
        <div className="space-y-6">
          {installCommands[activeOS].map((item, idx) => (
            <div key={idx} className="space-y-2">
              <div className="font-mono text-[10px] text-zinc-500 uppercase tracking-widest font-bold">
                {item.label}
              </div>
              <button
                onClick={() => handleCopy(item.cmd)}
                className="w-full flex items-center justify-between px-4 py-3 bg-zinc-200/30 dark:bg-zinc-950/20 border border-zinc-300 dark:border-zinc-850 font-mono text-xs md:text-sm text-zinc-850 dark:text-zinc-200 text-left hover:bg-zinc-200/50 dark:hover:bg-zinc-955/40 group/btn transition-colors cursor-pointer"
              >
                <span className="truncate select-text">{item.cmd}</span>
                <span className="flex-shrink-0 ml-4 font-mono text-[9px] uppercase font-bold text-zinc-400 group-hover/btn:text-blue-600 dark:group-hover/btn:text-blue-400 transition-colors">
                  {copiedText === item.cmd ? "Copied!" : item.cmd.startsWith("http") ? "Download" : "Copy"}
                </span>
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Getting Started / Verification Flow */}
      <h2 className="text-xl font-semibold font-switzer tracking-tight text-black dark:text-white border-b border-zinc-200 dark:border-zinc-800 pb-2 mb-6">
        Post-Installation Verification
      </h2>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-12">
        
        {/* Step 1 */}
        <div className="border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 p-5 flex flex-col justify-between min-h-[140px]">
          <div>
            <div className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400 w-fit mb-3">
              Step 01
            </div>
            <h3 className="font-switzer font-semibold text-sm md:text-base text-black dark:text-white mb-2">
              Verify Installation
            </h3>
            <p className="font-switzer font-medium text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed">
              Confirm that the paths were correctly configured by querying the CLI help menu.
            </p>
          </div>
          <button 
            onClick={() => handleCopy("bloc --help")}
            className="w-full flex items-center justify-between px-2 h-7 bg-zinc-200/50 dark:bg-zinc-900/50 hover:bg-zinc-200 dark:hover:bg-zinc-900 border border-zinc-300 dark:border-zinc-800 font-mono text-[9px] text-zinc-800 dark:text-zinc-200 cursor-pointer mt-4"
          >
            <span>bloc --help</span>
            <span className="text-zinc-400 font-bold uppercase">{copiedText === "bloc --help" ? "Copied" : "Copy"}</span>
          </button>
        </div>

        {/* Step 2 */}
        <div className="border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 p-5 flex flex-col justify-between min-h-[140px]">
          <div>
            <div className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400 w-fit mb-3">
              Step 02
            </div>
            <h3 className="font-switzer font-semibold text-sm md:text-base text-black dark:text-white mb-2">
              GitHub Login Link
            </h3>
            <p className="font-switzer font-medium text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed">
              Link your local terminal session with your Bloc Hub profile using secure GitHub OAuth.
            </p>
          </div>
          <button 
            onClick={() => handleCopy("bloc login")}
            className="w-full flex items-center justify-between px-2 h-7 bg-zinc-200/50 dark:bg-zinc-900/50 hover:bg-zinc-200 dark:hover:bg-zinc-900 border border-zinc-300 dark:border-zinc-800 font-mono text-[9px] text-zinc-800 dark:text-zinc-200 cursor-pointer mt-4"
          >
            <span>bloc login</span>
            <span className="text-zinc-400 font-bold uppercase">{copiedText === "bloc login" ? "Copied" : "Copy"}</span>
          </button>
        </div>

        {/* Step 3 */}
        <div className="border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 p-5 flex flex-col justify-between min-h-[140px]">
          <div>
            <div className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400 w-fit mb-3">
              Step 03
            </div>
            <h3 className="font-switzer font-semibold text-sm md:text-base text-black dark:text-white mb-2">
              Run Your First Recipe
            </h3>
            <p className="font-switzer font-medium text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed">
              Pull, configure, and boot up an optimized server for your hardware constraints.
            </p>
          </div>
          <button 
            onClick={() => {
              const testCmd = activeOS === "macos" 
                ? "bloc deploy arnav/deepseek-r1-8b-metal" 
                : "bloc deploy alice/qwen-7b-budget-beast";
              handleCopy(testCmd);
            }}
            className="w-full flex items-center justify-between px-2 h-7 bg-zinc-200/50 dark:bg-zinc-900/50 hover:bg-zinc-200 dark:hover:bg-zinc-900 border border-zinc-300 dark:border-zinc-800 font-mono text-[9px] text-zinc-800 dark:text-zinc-200 cursor-pointer mt-4"
          >
            <span className="truncate">{activeOS === "macos" ? "bloc deploy arnav/deepseek..." : "bloc deploy alice/qwen..."}</span>
            <span className="text-zinc-400 font-bold uppercase shrink-0">Copy</span>
          </button>
        </div>
      </div>

      {/* Hardware / Diagnostics Checklist */}
      <h2 className="text-xl font-semibold font-switzer tracking-tight text-black dark:text-white border-b border-zinc-200 dark:border-zinc-800 pb-2 mb-6">
        System Capabilities Checklist
      </h2>
      <div className="border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 rounded-none overflow-hidden mb-12">
        <table className="w-full text-left font-mono text-[11px] border-collapse">
          <thead>
            <tr className="border-b border-zinc-300 dark:border-zinc-800 bg-zinc-200/30 dark:bg-zinc-950/20 font-bold uppercase tracking-wider text-zinc-500 dark:text-zinc-400">
              <th className="px-4 py-3 w-1/4">System Type</th>
              <th className="px-4 py-3 w-1/3">Diagnostic Command</th>
              <th className="px-4 py-3">Dependencies & Requirements</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-zinc-200 dark:border-zinc-800">
              <td className="px-4 py-3 font-bold text-zinc-800 dark:text-zinc-200">Metal (macOS)</td>
              <td className="px-4 py-3 text-blue-600 dark:text-blue-400 select-text">xcode-select -p</td>
              <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">Apple Silicon chip (M-series), macOS Catalina+, and Xcode Command Line Tools.</td>
            </tr>
            <tr className="border-b border-zinc-200 dark:border-zinc-800">
              <td className="px-4 py-3 font-bold text-zinc-800 dark:text-zinc-200">CUDA (Linux / Win)</td>
              <td className="px-4 py-3 text-blue-600 dark:text-blue-400 select-text">nvidia-smi</td>
              <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">NVIDIA Graphics card (compute capability 6.0+), latest graphics drivers installed.</td>
            </tr>
            <tr>
              <td className="px-4 py-3 font-bold text-zinc-800 dark:text-zinc-200">CPU Fallback</td>
              <td className="px-4 py-3 text-blue-600 dark:text-blue-400 select-text">sysctl hw.optional.avx2</td>
              <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">Universal fallback. Recommends at least 16GB CPU RAM and AVX2 instruction support.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}
