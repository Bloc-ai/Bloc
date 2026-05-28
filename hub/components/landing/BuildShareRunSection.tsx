"use client";

import { useState, useRef, useEffect } from "react";

// 3D Cuboid SVG — shared between all three steps
function Cuboid({ onClick }: { onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      className="w-full mt-8 relative aspect-[320/220] min-h-[220px] group cursor-pointer select-none"
    >
      {/* 3D Socket Base (stationary) */}
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

      {/* 3D Keycap */}
      <div
        className="absolute inset-0 w-full h-full transform transition-all duration-300 group-hover:-translate-x-3 group-hover:-translate-y-3 active:translate-x-0 active:translate-y-0 ease-out z-10"
        style={{ transitionTimingFunction: "cubic-bezier(0.34, 1.56, 0.64, 1)" }}
      >
        <svg
          viewBox="0 0 320 220"
          className="w-full h-full drop-shadow-[0_4px_6px_rgba(0,0,0,0.03)] dark:drop-shadow-[0_4px_6px_rgba(0,0,0,0.3)] group-hover:drop-shadow-[0_12px_24px_rgba(0,0,0,0.08)] dark:group-hover:drop-shadow-[0_12px_24px_rgba(0,0,0,0.5)] transition-all duration-300"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <polygon
            points="310,0 320,10 320,220 310,210"
            className="fill-[#ededeb]/90 dark:fill-[#201f1f]/90 stroke-zinc-300 dark:stroke-zinc-800 transition-colors"
            strokeWidth="1"
          />
          <polygon
            points="0,210 10,220 320,220 310,210"
            className="fill-[#e5e5e0]/90 dark:fill-[#1b1a1a]/90 stroke-zinc-300 dark:stroke-zinc-800 transition-colors"
            strokeWidth="1"
          />
          <rect
            x="0" y="0" width="310" height="210"
            className="fill-[#f6f6f3] dark:fill-[#171616] stroke-zinc-300 dark:stroke-zinc-800 transition-colors"
            strokeWidth="1"
          />
          <path
            d="M 0,0 L 310,0 L 320,10 L 320,220 L 10,220 L 0,210 Z"
            className="stroke-black dark:stroke-white transition-colors"
            strokeWidth="1.5"
            strokeLinejoin="round"
            fill="none"
          />
        </svg>
      </div>
    </div>
  );
}

const STEPS = [
  {
    num: "01",
    title: "Build",
    desc: "Create a reproducible recipe with your models, configs, runtimes, and optimizations.",
  },
  {
    num: "02",
    title: "Share",
    desc: "Publish to Bloc Hub and share with the community.",
  },
  {
    num: "03",
    title: "Run",
    desc: "Anyone can deploy the exact same environment with one command.",
  },
];

export default function BuildShareRunSection() {
  const [activeStep, setActiveStep] = useState<number | null>(null);
  // H5 Fix: cache Audio in a ref — instantiated once on mount
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
    <div className="relative w-full mt-16 select-none">
      {/* Horizontal connector line */}
      <div
        className={`absolute top-4.5 left-8 right-8 h-[1px] bg-zinc-300 dark:bg-zinc-800 hidden md:block z-0 transition-opacity duration-500 ease-in-out ${
          activeStep !== null ? "opacity-0" : "opacity-100"
        }`}
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-12 md:gap-8 w-full relative z-10">
        {STEPS.map((step, i) => (
          <div
            key={step.num}
            onMouseEnter={() => setActiveStep(i + 1)}
            onMouseLeave={() => setActiveStep(null)}
            className={`flex flex-col items-start w-full transition-all duration-500 ease-in-out ${
              activeStep !== null && activeStep !== i + 1
                ? "opacity-0 scale-[0.97] pointer-events-none"
                : "opacity-100 scale-100"
            }`}
          >
            {/* Step Circle */}
            <div className="w-9 h-9 rounded-full flex items-center justify-center text-xs font-mono font-bold border border-zinc-300 dark:border-zinc-800 bg-zinc-100 dark:bg-zinc-900 text-zinc-900 dark:text-zinc-100 z-10 shadow-sm">
              {step.num}
            </div>
            <h3 className="text-2xl font-semibold tracking-tight font-switzer text-black dark:text-white mt-6 mb-2">
              {step.title}
            </h3>
            <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm leading-relaxed max-w-sm min-h-[48px]">
              {step.desc}
            </p>

            <Cuboid onClick={playSwitchSound} />
          </div>
        ))}
      </div>
    </div>
  );
}
