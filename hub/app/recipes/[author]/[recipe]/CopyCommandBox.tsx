"use client";

import { useState } from "react";

export default function CopyCommandBox({ recipeId }: { recipeId: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(`bloc deploy ${recipeId}`);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div className="max-w-xl w-full">
      <button 
        onClick={handleCopy}
        className="w-full flex items-center justify-between px-4 h-10 bg-zinc-200/50 dark:bg-zinc-900/50 hover:bg-zinc-200 dark:hover:bg-zinc-900 border border-zinc-300 dark:border-zinc-800 font-mono text-xs text-zinc-800 dark:text-zinc-200 cursor-pointer active:scale-[0.98] transition-all"
      >
        <span className="truncate select-text">bloc deploy {recipeId}</span>
        <span className="flex-shrink-0 ml-4 font-bold uppercase text-[10px] text-zinc-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors">
          {copied ? "Copied!" : "Copy"}
        </span>
      </button>
    </div>
  );
}
