"use client";

import { useState, useEffect } from "react";

const TERMINAL_LINES = [
  { text: "bloc deploy alice/qwen-7b-budget-beast", delay: 1000, isCommand: true },
  { text: "[info] Probing system capabilities...", delay: 800 },
  { text: "[info] Detected GPU: NVIDIA RTX 3050 (8GB VRAM)", delay: 800 },
  { text: "[info] Downloading manifest from hub...", delay: 600 },
  { text: "[info] Caching GGUF model weights... [100%]", delay: 1200 },
  { text: "[info] Booting llama.cpp engine...", delay: 800 },
  { text: "[info] 28/32 layers offloaded to VRAM", delay: 600 },
  { text: "[success] API server running at http://127.0.0.1:8080/v1", delay: 1800, isSuccess: true },
  { text: "[info] Ready for requests. Press Ctrl+C to shutdown.", delay: 2000 },
];

export default function TerminalDemo() {
  const [visibleLines, setVisibleLines] = useState<typeof TERMINAL_LINES>([]);
  const [currentIndex, setCurrentIndex] = useState(0);

  useEffect(() => {
    if (currentIndex < TERMINAL_LINES.length) {
      const timer = setTimeout(() => {
        setVisibleLines((prev) => [...prev, TERMINAL_LINES[currentIndex]]);
        setCurrentIndex((prev) => prev + 1);
      }, TERMINAL_LINES[currentIndex].delay);
      return () => clearTimeout(timer);
    } else {
      const resetTimer = setTimeout(() => {
        setVisibleLines([]);
        setCurrentIndex(0);
      }, 5000);
      return () => clearTimeout(resetTimer);
    }
  }, [currentIndex]);

  return (
    <aside className="bg-black text-white p-6 rounded-lg w-full max-w-lg h-[330px] font-mono text-[11px] leading-relaxed relative overflow-hidden select-none text-left">
      {/* Terminal Window Header */}
      <div className="flex justify-between items-center">
        <div className="flex space-x-2">
          <div className="w-3 h-3 rounded-full bg-red-500"></div>
          <div className="w-3 h-3 rounded-full bg-yellow-500"></div>
          <div className="w-3 h-3 rounded-full bg-green-500"></div>
        </div>
        <p className="text-xs text-zinc-500">bash</p>
      </div>

      {/* Terminal Content */}
      <div className="mt-4 space-y-1.5">
        {visibleLines.map((line, idx) => {
          if (line.isCommand) {
            return (
              <p key={idx} className="text-green-400">
                $ {line.text}
              </p>
            );
          }
          if (line.isSuccess) {
            return (
              <p key={idx} className="text-emerald-400 font-bold">
                {line.text}
              </p>
            );
          }
          return (
            <p key={idx} className="text-white">
              {line.text}
            </p>
          );
        })}
        {/* Blinking prompt cursor */}
        <div className="flex items-center text-green-400">
          <span>$</span>
          <span className="w-1.5 h-3.5 bg-green-400 animate-pulse ml-1.5 inline-block" />
        </div>
      </div>
    </aside>
  );
}
