"use client";

import React, { useState } from "react";

interface TerminalCommandProps {
  command?: string;
  className?: string;
}

export default function TerminalCommand({
  command = "bloc deploy arnav/qwen-3.5-9b-super",
  className = "",
}: TerminalCommandProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error("Failed to copy: ", err);
    }
  };

  return (
    <div className={`group relative flex items-center justify-between gap-4 bg-[#111110] border border-zinc-800/80 rounded-none px-5 py-3 font-mono text-[14px] leading-none text-zinc-200 select-none w-full max-w-md shadow-lg ${className}`}>
      {/* Corner L-brackets appearing on hover */}
      {/* Top-Left */}
      <svg 
        viewBox="0 0 12 12" 
        className="absolute top-0 left-0 w-2 h-2 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5"
      >
        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
      </svg>
      {/* Top-Right */}
      <svg 
        viewBox="0 0 12 12" 
        className="absolute top-0 right-0 w-2 h-2 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5"
      >
        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
      </svg>
      {/* Bottom-Left */}
      <svg 
        viewBox="0 0 12 12" 
        className="absolute bottom-0 left-0 w-2 h-2 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5"
      >
        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
      </svg>
      {/* Bottom-Right */}
      <svg 
        viewBox="0 0 12 12" 
        className="absolute bottom-0 right-0 w-2 h-2 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5"
      >
        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
      </svg>

      <div className="flex items-center gap-3 overflow-x-auto scrollbar-none">
        <span className="text-zinc-600 select-none">$</span>
        <span className="whitespace-nowrap tracking-wide text-zinc-100">{command}</span>
      </div>
      
      <button
        onClick={handleCopy}
        className="text-zinc-500 hover:text-zinc-300 transition-colors p-1 cursor-pointer flex-shrink-0"
        title="Copy to clipboard"
      >
        {copied ? (
          // Active checkmark icon
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="none"
            stroke="#22c55e"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <polyline points="20 6 9 17 4 12" />
          </svg>
        ) : (
          // Normal copy double-sheet icon
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
          </svg>
        )}
      </button>
    </div>
  );
}
