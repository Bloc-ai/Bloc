"use client";

import { useState, useRef, useEffect } from "react";

interface CardData {
  title: string;
  desc: string;
  link: string;
}

const CARDS: CardData[] = [
  {
    title: "Local AI Recipes",
    desc: "Browse, customize, and share optimized llama.cpp recipes configured for different host hardware. Fully version-controlled and reproducible.",
    link: "https://github.com/Bloc-ai/Bloc/tree/main/recipes"
  },
  {
    title: "Next.js Web Hub",
    desc: "A collaborative registry and web interface to explore community-built setups, track model downloads, and manage remote endpoints.",
    link: "https://github.com/Bloc-ai/Bloc/tree/main/hub"
  },
  {
    title: "Developer CLI Tool",
    desc: "A lightweight terminal client to run, benchmark, and serve local AI environments instantly with a single offline command.",
    link: "https://github.com/Bloc-ai/Bloc/tree/main/cli"
  }
];

interface CommunityCuboidKeyProps {
  card: CardData;
  playSound: () => void;
  state: "raised" | "flat" | "pressed";
  onMouseEnter: () => void;
  onMouseLeave: () => void;
}

function CommunityCuboidKey({
  card,
  playSound,
  state,
  onMouseEnter,
  onMouseLeave
}: CommunityCuboidKeyProps) {
  const [isClickPressed, setIsClickPressed] = useState(false);

  const handlePress = () => {
    if (isClickPressed) return;
    setIsClickPressed(true);
    playSound();
    
    // Play physical depression animation click
    setTimeout(() => {
      setIsClickPressed(false);
      window.open(card.link, "_blank", "noopener,noreferrer");
    }, 180);
  };

  // Determine if it should display as 3D raised or flat 2D
  const is3D = state === "raised" && !isClickPressed;

  const transformClasses = is3D
    ? "-translate-x-3 -translate-y-3"
    : "translate-x-0 translate-y-0";

  const shadowClasses = is3D
    ? "drop-shadow-[0_12px_24px_rgba(0,0,0,0.08)] dark:drop-shadow-[0_12px_24px_rgba(0,0,0,0.5)]"
    : "drop-shadow-[0_4px_6px_rgba(0,0,0,0.03)] dark:drop-shadow-[0_4px_6px_rgba(0,0,0,0.3)]";

  return (
    <div
      onClick={handlePress}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      className="w-full relative aspect-[320/220] min-h-[220px] group cursor-pointer select-none"
    >
      {/* 3D Socket Base (stationary behind the keycap) */}
      <div className="absolute inset-0 w-full h-full pointer-events-none">
        <svg
          viewBox="0 0 320 220"
          className="w-full h-full opacity-60 dark:opacity-40"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <rect
            x="0" y="0" width="310" height="210"
            className="stroke-zinc-300 dark:stroke-zinc-800 fill-zinc-200/20 dark:fill-zinc-950/20"
            strokeWidth="1"
            strokeDasharray="4 4"
          />
          <g className="stroke-zinc-300 dark:stroke-zinc-800 opacity-60" strokeWidth="1.5">
            <circle cx="155" cy="105" r="16" fill="none" />
            <path d="M 147,105 L 163,105 M 155,97 L 155,113" />
          </g>
        </svg>
      </div>

      {/* 3D Depressable Keycap */}
      <div
        className={`absolute inset-0 w-full h-full transform transition-all duration-300 ease-out z-10 ${transformClasses}`}
        style={{ transitionTimingFunction: "cubic-bezier(0.34, 1.56, 0.64, 1)" }}
      >
        <svg
          viewBox="0 0 320 220"
          className={`w-full h-full transition-all duration-300 ${shadowClasses}`}
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          {/* Right bevel */}
          <polygon
            points="310,0 320,10 320,220 310,210"
            className={`fill-[#ededeb]/90 dark:fill-[#201f1f]/90 stroke-zinc-300 dark:stroke-zinc-800 transition-all duration-300 ${
              is3D ? "opacity-100" : "opacity-0 pointer-events-none"
            }`}
            strokeWidth="1"
          />
          {/* Bottom bevel */}
          <polygon
            points="0,210 10,220 320,220 310,210"
            className={`fill-[#e5e5e0]/90 dark:fill-[#1b1a1a]/90 stroke-zinc-300 dark:stroke-zinc-800 transition-all duration-300 ${
              is3D ? "opacity-100" : "opacity-0 pointer-events-none"
            }`}
            strokeWidth="1"
          />
          {/* Front Face */}
          <rect
            x="0" y="0" width="310" height="210"
            className="fill-[#f6f6f3] dark:fill-[#171616] stroke-zinc-300 dark:stroke-zinc-800 transition-colors"
            strokeWidth="1"
          />
          
          {/* 3D Outline */}
          <path
            d="M 0,0 L 310,0 L 320,10 L 320,220 L 10,220 L 0,210 Z"
            className={`stroke-black dark:stroke-white transition-all duration-300 ${
              is3D ? "opacity-100" : "opacity-0"
            }`}
            strokeWidth="1.5"
            strokeLinejoin="round"
            fill="none"
          />

          {/* 2D Outline (Morphs to flat rectangle outline when the other keys are hovered) */}
          <rect
            x="0" y="0" width="310" height="210"
            className={`stroke-black dark:stroke-white transition-all duration-300 ${
              is3D ? "opacity-0" : "opacity-100"
            }`}
            strokeWidth="1.5"
            fill="none"
          />
        </svg>

        {/* Content overlaid perfectly on the front face (rect is 310x210 inside the 320x220 viewBox) */}
        <div
          className="absolute top-0 left-0 p-6 flex flex-col justify-between text-left"
          style={{
            width: "96.875%", // 310 / 320
            height: "95.454%", // 210 / 220
          }}
        >
          <div className="flex items-start justify-between gap-4">
            <h3 className="font-switzer font-semibold text-base md:text-lg text-black dark:text-white leading-tight">
              {card.title}
            </h3>
            <div className="shrink-0">
              <svg 
                className="w-6.5 h-6.5 text-zinc-400 dark:text-zinc-600 group-hover:text-black dark:group-hover:text-white transition-colors duration-200" 
                viewBox="0 0 24 24" 
                fill="currentColor"
              >
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
              </svg>
            </div>
          </div>
          <p className="font-switzer font-medium text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed mt-4">
            {card.desc}
          </p>
        </div>
      </div>
    </div>
  );
}

export default function CommunityCuboids() {
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  useEffect(() => {
    audioRef.current = new Audio("/images/switch-sound.mp3");
  }, []);

  const playSwitchSound = () => {
    if (audioRef.current) {
      audioRef.current.currentTime = 0;
      audioRef.current.play().catch(() => {});
    }
  };

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6 mt-8 w-full">
      {CARDS.map((card, idx) => {
        // Determine the state based on what index is currently hovered
        let state: "raised" | "flat" | "pressed" = "raised";
        if (hoveredIndex !== null) {
          state = hoveredIndex === idx ? "raised" : "flat";
        }

        return (
          <CommunityCuboidKey
            key={card.title}
            card={card}
            playSound={playSwitchSound}
            state={state}
            onMouseEnter={() => setHoveredIndex(idx)}
            onMouseLeave={() => setHoveredIndex(null)}
          />
        );
      })}
    </div>
  );
}
