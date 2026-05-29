// Server Component — no "use client" directive
import ShortcutButton from "@/components/ShortcutButton";
import TerminalCommand from "@/components/TerminalCommand";
import FaqAccordion from "@/components/FaqAccordion";
import Link from "next/link";
import { AsciiCanvas } from "@/components/AsciiCanvas";
import Image from "next/image";
import { Cpu, Box, Code2, Layers, Activity, FileCode2, Shield } from "lucide-react";
import TerminalDemo from "@/components/landing/TerminalDemo";
import BuildShareRunSection from "@/components/landing/BuildShareRunSection";

function HighlightText({ children }: { children: React.ReactNode }) {
  return (
    <span 
      className="inline text-black dark:text-white px-2 box-decoration-clone"
      style={{
        fontFamily: 'inherit',
        background: 'linear-gradient(to bottom, transparent 15%, #2563EB 15%, #2563EB 82%, transparent 82%)'
      }}
    >
      {children}
    </span>
  );
}

const bentoRow1 = [
  {
    title: "Auto-Hardware Prober",
    desc: "Automatically checks CPU instructions, VRAM thresholds, and GPU capabilities to guide configuration options.",
    icon: <Cpu className="w-5 h-5 text-blue-500" />
  },
  {
    title: "Zero-Dependency Setup",
    desc: "Runtimes, libraries, and optimized model runners are pre-packaged and managed dynamically without manual build config.",
    icon: <Box className="w-5 h-5 text-blue-500" />
  },
  {
    title: "OpenAI-Compatible Endpoint",
    desc: "Instantly serves endpoints at localhost:8080/v1. Drop in existing chat UI SDKs, LangChain, or Autogen scripts.",
    icon: <Code2 className="w-5 h-5 text-blue-500" />
  }
];

const bentoRow2 = [
  {
    title: "Granular Layer Offloading",
    desc: "Control layers in VRAM vs RAM to balance speed and memory usage.",
    icon: <Layers className="w-5 h-5 text-blue-500" />
  },
  {
    title: "Telemetry Benchmarks",
    desc: "Opt-in speed profiles verify tokens/sec across specific host chips.",
    icon: <Activity className="w-5 h-5 text-blue-500" />
  },
  {
    title: "Multi-Format Support",
    desc: "Compatible with GGUF, EXL2, AWQ, and custom quantized manifests.",
    icon: <FileCode2 className="w-5 h-5 text-blue-500" />
  },
  {
    title: "Offline Sovereignty",
    desc: "Runs 100% offline, guaranteeing no text data leaves the host environment.",
    icon: <Shield className="w-5 h-5 text-blue-500" />
  }
];

const communityCards = [
  {
    title: "Local AI Recipes",
    desc: "Browse, customize, and share optimized llama.cpp recipes configured for different host hardware. Fully version-controlled and reproducible.",
    link: "https://github.com/arnav080/Bloc/tree/main/recipes"
  },
  {
    title: "Next.js Web Hub",
    desc: "A collaborative registry and web interface to explore community-built setups, track model downloads, and manage remote endpoints.",
    link: "https://github.com/arnav080/Bloc/tree/main/hub"
  },
  {
    title: "Developer CLI Tool",
    desc: "A lightweight terminal client to deploy, benchmark, and serve local AI environments instantly with a single offline command.",
    link: "https://github.com/arnav080/Bloc/tree/main/cli"
  }
];

export default function Home() {

  return (
    <div className="max-w-6xl w-full mx-auto px-4 pb-24">
      
      {/* Hero Viewport Wrapper */}
      <div className="w-full min-h-[calc(100vh-48px)] flex flex-col justify-center items-center">
        {/* Hero Box Container */}
        <div className="relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] rounded-none">
          
          {/* SVG Corner L-Brackets */}
          {/* Top-Left */}
          <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white pointer-events-none">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          {/* Top-Right */}
          <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          {/* Bottom-Left */}
          <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          {/* Bottom-Right */}
          <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>

          {/* Section 1: Single Line Feature List */}
          <div className="w-full min-h-[48px] flex flex-wrap items-center justify-center gap-3 sm:gap-6 py-3 px-4 text-center text-[10px] sm:text-xs font-mono text-zinc-500 dark:text-zinc-400">
            <span>
              <strong className="text-black dark:text-white font-bold">1-command</strong> deployments
            </span>
            <span className="text-zinc-300 dark:text-zinc-700">·</span>
            <span>
              <strong className="text-black dark:text-white font-bold">100% local</strong> and open-source
            </span>
            <span className="text-zinc-300 dark:text-zinc-700">·</span>
            <span>
              <strong className="text-black dark:text-white font-bold">Community-powered</strong> AI infrastructure
            </span>
          </div>

          {/* Divider 1 with T-Brackets */}
          <div className="border-t border-zinc-300 dark:border-zinc-800 relative">
            {/* Left T-Bracket */}
            <svg viewBox="0 0 12 16" className="absolute -left-[1px] top-0 -translate-y-1/2 w-3 h-4 fill-black dark:fill-white pointer-events-none">
              <path d="M 0,0 L 1,0 L 1,4 Q 1,7 4,7 L 12,7 L 12,8 L 4,8 Q 1,8 1,11 L 1,16 L 0,16 Z" />
            </svg>
            {/* Right T-Bracket */}
            <svg viewBox="0 0 12 16" className="absolute -right-[1px] top-0 -translate-y-1/2 w-3 h-4 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
              <path d="M 0,0 L 1,0 L 1,4 Q 1,7 4,7 L 12,7 L 12,8 L 4,8 Q 1,8 1,11 L 1,16 L 0,16 Z" />
            </svg>
          </div>

          {/* Section 2: Main Hero */}
          <div className="w-full min-h-[380px] flex flex-col items-center justify-center py-16 px-6">
            {/* Hero Heading */}
            <h1 className="text-4xl sm:text-5xl md:text-7xl font-medium tracking-tight text-center max-w-5xl leading-[1.3] font-switzer text-black dark:text-white mb-8">
              <HighlightText>
                The huggingface For <br className="hidden md:inline" /> Local AI Deployments.
              </HighlightText>
            </h1>

            {/* Subheading Paragraph */}
            <p className="text-center text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-xl leading-snug">
              Discover and deploy production-ready local AI recipes in minutes. Run identical llama.cpp environments shared by the community with one command. Any model, any hardware. Fully local and reproducible.
            </p>

            {/* Action Buttons */}
            <div className="flex flex-wrap items-center justify-center gap-4 mt-8">
              <ShortcutButton text="Explore" shortcutKey="E" href="/registry" variant="black" />
              <ShortcutButton text="Documentation" shortcutKey="D" href="/docs" variant="white" />
            </div>

            {/* Terminal Command Box */}
            <div className="mt-8">
              <TerminalCommand command="bloc deploy arnav/qwen-3.5-9b-super" />
            </div>
          </div>

          {/* Divider 2 with T-Brackets */}
          <div className="border-t border-zinc-300 dark:border-zinc-800 relative">
            {/* Left T-Bracket */}
            <svg viewBox="0 0 12 16" className="absolute -left-[1px] top-0 -translate-y-1/2 w-3 h-4 fill-black dark:fill-white pointer-events-none">
              <path d="M 0,0 L 1,0 L 1,4 Q 1,7 4,7 L 12,7 L 12,8 L 4,8 Q 1,8 1,11 L 1,16 L 0,16 Z" />
            </svg>
            {/* Right T-Bracket */}
            <svg viewBox="0 0 12 16" className="absolute -right-[1px] top-0 -translate-y-1/2 w-3 h-4 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
              <path d="M 0,0 L 1,0 L 1,4 Q 1,7 4,7 L 12,7 L 12,8 L 4,8 Q 1,8 1,11 L 1,16 L 0,16 Z" />
            </svg>
          </div>

          {/* Section 3: Single Row Partner Logos */}
          <div className="w-full min-h-[72px] flex flex-wrap items-center justify-center gap-x-8 gap-y-6 md:gap-x-12 py-5 px-6 overflow-hidden">
            {/* Alibaba Cloud */}
            <div className="group relative h-6 w-24 md:w-28 flex-shrink-0 grayscale hover:grayscale-0 opacity-60 dark:opacity-40 hover:opacity-100 dark:hover:opacity-100 transition-all duration-300">
              <Image 
                src="/images/alibaba-cloud-logo.svg" 
                alt="Alibaba Cloud Logo" 
                fill
                className="object-contain dark:invert group-hover:dark:invert-0 transition-all duration-300" 
              />
            </div>
            {/* Meta */}
            <div className="group relative h-6 w-24 md:w-28 flex-shrink-0 grayscale hover:grayscale-0 opacity-60 dark:opacity-40 hover:opacity-100 dark:hover:opacity-100 transition-all duration-300">
              <Image 
                src="/images/meta-logo.svg" 
                alt="Meta Logo" 
                fill
                className="object-contain dark:invert group-hover:dark:invert-0 transition-all duration-300" 
              />
            </div>
            {/* Qwen AI */}
            <div className="group relative h-6 w-24 md:w-28 scale-130 origin-center flex-shrink-0 grayscale hover:grayscale-0 opacity-60 dark:opacity-40 hover:opacity-100 dark:hover:opacity-100 transition-all duration-300">
              <Image 
                src="/images/qwen-ai-logo.svg" 
                alt="Qwen AI Logo" 
                fill
                className="object-contain dark:invert group-hover:dark:invert-0 transition-all duration-300" 
              />
            </div>
            {/* Mistral AI */}
            <div className="group relative h-6 w-24 md:w-28 flex-shrink-0 grayscale hover:grayscale-0 opacity-60 dark:opacity-40 hover:opacity-100 dark:hover:opacity-100 transition-all duration-300">
              <Image 
                src="/images/mistral-ai-logo.svg" 
                alt="Mistral AI Logo" 
                fill
                className="object-contain dark:invert group-hover:dark:invert-0 transition-all duration-300" 
              />
            </div>
            {/* MiniMax */}
            <div className="group relative h-6 w-24 md:w-28 flex-shrink-0 grayscale hover:grayscale-0 opacity-60 dark:opacity-40 hover:opacity-100 dark:hover:opacity-100 transition-all duration-300">
              <Image 
                src="/images/minimax-logo.svg" 
                alt="MiniMax Logo" 
                fill
                className="object-contain dark:invert group-hover:dark:invert-0 transition-all duration-300" 
              />
            </div>
          </div>

        </div>
      </div>

      {/* Second Box Container */}
      <div className="relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] rounded-none mt-44">
        
        {/* SVG Corner L-Brackets */}
        {/* Top-Left */}
        <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white pointer-events-none">
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        {/* Top-Right */}
        <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        {/* Bottom-Left */}
        <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none">
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        {/* Bottom-Right */}
        <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none">
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>

        {/* Section 1: Top Section */}
        <div className="w-full min-h-[48px] flex items-center justify-start py-2 px-6">
          <h2 className="font-switzer font-medium text-base md:text-lg text-[#171616] dark:text-white">
            What is Bloc?
          </h2>
        </div>

        {/* Divider with T-Brackets */}
        <div className="border-t border-zinc-300 dark:border-zinc-800 relative">
          {/* Left T-Bracket */}
          <svg viewBox="0 0 12 16" className="absolute -left-[1px] top-0 -translate-y-1/2 w-3 h-4 fill-black dark:fill-white pointer-events-none">
            <path d="M 0,0 L 1,0 L 1,4 Q 1,7 4,7 L 12,7 L 12,8 L 4,8 Q 1,8 1,11 L 1,16 L 0,16 Z" />
          </svg>
          {/* Right T-Bracket */}
          <svg viewBox="0 0 12 16" className="absolute -right-[1px] top-0 -translate-y-1/2 w-3 h-4 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
            <path d="M 0,0 L 1,0 L 1,4 Q 1,7 4,7 L 12,7 L 12,8 L 4,8 Q 1,8 1,11 L 1,16 L 0,16 Z" />
          </svg>
        </div>

        {/* Section 2: Bottom Section */}
        <div className="w-full py-6 px-6 text-left">
          <div className="grid grid-cols-1 md:grid-cols-10 gap-8 items-stretch w-full">
            {/* Left: Text (6/10 width) */}
            <div className="md:col-span-6 flex flex-col justify-center">
              <p className="font-switzer font-medium text-black dark:text-white text-base md:text-lg leading-relaxed">
                Bloc Hub is an open platform for sharing and deploying reproducible local AI environments. Developers can publish complete llama.cpp recipes including models, quantization settings, runtimes, GPU optimizations, and configurations, while others can instantly run the exact same setup using a single CLI command. Instead of manually recreating local AI stacks from scattered guides and configs, Bloc Hub makes deployments portable, reproducible, and community-driven.
              </p>
            </div>

            {/* Right: Terminal Live Demo Animation (4/10 width) */}
            <div className="md:col-span-4">
              <TerminalDemo />
            </div>
          </div>
        </div>

      </div>

      {/* Heading and Subheading Section (Left-aligned) */}
      <div className="w-full text-left mt-44 px-2 self-start">
        <h2 className="text-4xl md:text-5xl font-medium tracking-tight font-switzer text-black dark:text-white mb-6">
          <HighlightText>Build, Share, Run</HighlightText> – instantly
        </h2>
        <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-3xl leading-relaxed">
          Bloc Hub helps teams share reproducible local AI deployments from prototype to production. Discover community-built llama.cpp recipes, deploy identical environments instantly, and continuously improve performance across models, hardware, and runtimes.
        </p>

        {/* Build, Share, Run Three-Step Flow Layout — client island */}
        <BuildShareRunSection />
      </div>

      {/* Fourth Section: Everything needed to run local AI */}
      <div className="w-full text-left mt-44 px-2 self-start">
        <h2 className="text-4xl md:text-5xl font-medium tracking-tight font-switzer text-black dark:text-white mb-6">
          Everything needed to run <HighlightText>local AI.</HighlightText>
        </h2>
        <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-3xl leading-relaxed">
          Deploy community-built llama.cpp recipes, reproduce exact environments, and optimize performance from prototype to production.
        </p>        {/* Bento Grid */}
        <div className="flex flex-col gap-4 mt-8 w-full">
          {/* Row 1: 3 Columns */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 w-full">
            {bentoRow1.map((item) => (
              <div key={item.title} className="relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 rounded-none p-5 flex flex-col justify-between min-h-[160px]">
                <div className="flex items-start justify-between gap-4">
                  <h3 className="font-switzer font-semibold text-sm md:text-base text-black dark:text-white leading-tight">
                    {item.title}
                  </h3>
                  <div className="shrink-0 p-1 border border-zinc-200 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#1c1b1b] rounded-none">
                    {item.icon}
                  </div>
                </div>
                <p className="font-switzer font-medium text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed mt-4">
                  {item.desc}
                </p>
              </div>
            ))}
          </div>

          {/* Row 2: 4 Columns */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 w-full">
            {bentoRow2.map((item) => (
              <div key={item.title} className="relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 rounded-none p-5 flex flex-col justify-between min-h-[160px]">
                <div className="flex items-start justify-between gap-4">
                  <h3 className="font-switzer font-semibold text-sm md:text-base text-black dark:text-white leading-tight">
                    {item.title}
                  </h3>
                  <div className="shrink-0 p-1 border border-zinc-200 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#1c1b1b] rounded-none">
                    {item.icon}
                  </div>
                </div>
                <p className="font-switzer font-medium text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed mt-4">
                  {item.desc}
                </p>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Fifth Section: Ready-to-deploy local AI setups */}
      <div className="w-full text-left mt-44 px-2 self-start">
        <h2 className="text-4xl md:text-5xl font-medium tracking-tight font-switzer text-black dark:text-white mb-6">
          <HighlightText>Ready-to-deploy</HighlightText> local AI setups
        </h2>
        <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-3xl leading-relaxed">
          Explore curated local AI recipes optimized for different models, GPUs, runtimes, and workloads deployable instantly with a single command.
        </p>

        {/* 4 Stacked Recipe List Blocks */}
        <div className="flex flex-col gap-4 mt-8 w-full">
          {/* Block 1 */}
          <div className="group relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] rounded-none min-h-[48px] transition-colors duration-200 hover:bg-[#ededeb] dark:hover:bg-[#201f1f] cursor-pointer">
            <svg 
              viewBox="0 0 12 12" 
              className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute bottom-0 left-0 w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute bottom-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <div className="w-full min-h-[48px]"></div>
          </div>

          {/* Block 2 */}
          <div className="group relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] rounded-none min-h-[48px] transition-colors duration-200 hover:bg-[#ededeb] dark:hover:bg-[#201f1f] cursor-pointer">
            <svg 
              viewBox="0 0 12 12" 
              className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute bottom-0 left-0 w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute bottom-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <div className="w-full min-h-[48px]"></div>
          </div>

          {/* Block 3 */}
          <div className="group relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] rounded-none min-h-[48px] transition-colors duration-200 hover:bg-[#ededeb] dark:hover:bg-[#201f1f] cursor-pointer">
            <svg 
              viewBox="0 0 12 12" 
              className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute bottom-0 left-0 w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute bottom-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <div className="w-full min-h-[48px]"></div>
          </div>

          {/* Block 4 */}
          <div className="group relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] rounded-none min-h-[48px] transition-colors duration-200 hover:bg-[#ededeb] dark:hover:bg-[#201f1f] cursor-pointer">
            <svg 
              viewBox="0 0 12 12" 
              className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute bottom-0 left-0 w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg 
              viewBox="0 0 12 12" 
              className="absolute bottom-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5"
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <div className="w-full min-h-[48px]"></div>
          </div>
        </div>

        {/* Subtext and Explore CTA Button */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-8 mt-8 w-full">
          <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-3xl leading-relaxed">
            New recipes are added constantly by developers, researchers, and local AI enthusiasts. Browse the full collection and find the setup that fits your workflow.
          </p>
          <div className="flex-shrink-0 self-start md:self-center">
            <ShortcutButton text="Explore" shortcutKey="E" href="/registry" variant="black" />
          </div>
        </div>
      </div>

      {/* Sixth Section: Built by the local AI community */}
      <div className="w-full text-left mt-44 px-2 self-start">
        <h2 className="text-4xl md:text-5xl font-medium tracking-tight font-switzer text-black dark:text-white mb-6">
          Built by the local AI <HighlightText>community.</HighlightText>
        </h2>
        <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-3xl leading-relaxed">
          Share your own recipes, improve existing deployments, and collaborate on open infrastructure powering the next generation of local AI.
        </p>

        {/* 3 Blocks in a row (Grid) */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6 mt-8 w-full">
          {communityCards.map((card) => (
            <a 
              key={card.title} 
              href={card.link}
              target="_blank"
              rel="noopener noreferrer"
              className="group relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] rounded-none min-h-[180px] p-6 flex flex-col justify-between transition-all duration-200 hover:bg-[#ededeb] dark:hover:bg-[#201f1f] hover:border-zinc-400 dark:hover:border-zinc-700 cursor-pointer block text-current no-underline"
            >
              {/* SVG Corner L-Brackets */}
              <svg 
                viewBox="0 0 12 12" 
                className="absolute -top-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out group-hover:-translate-x-1 group-hover:-translate-y-1"
              >
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg 
                viewBox="0 0 12 12" 
                className="absolute -top-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out group-hover:translate-x-1 group-hover:-translate-y-1"
              >
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg 
                viewBox="0 0 12 12" 
                className="absolute -bottom-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out group-hover:-translate-x-1 group-hover:translate-y-1"
              >
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>
              <svg 
                viewBox="0 0 12 12" 
                className="absolute -bottom-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out group-hover:translate-x-1 group-hover:translate-y-1"
              >
                <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
              </svg>

              <div className="flex items-start justify-between gap-4">
                <h3 className="font-switzer font-semibold text-base text-black dark:text-white leading-tight">
                  {card.title}
                </h3>
                <div className="shrink-0">
                  <svg className="w-6.5 h-6.5 text-white fill-white hover:text-zinc-200 transition-colors duration-200" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                  </svg>
                </div>
              </div>
              <p className="font-switzer font-medium text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed mt-4">
                {card.desc}
              </p>
            </a>
          ))}
        </div>
      </div>

      {/* Call-to-Action Section */}
      <div className="w-full flex flex-col md:flex-row md:items-center justify-between gap-8 mt-44 px-2 self-start">
        {/* Left Side: Heading */}
        <div className="flex-grow text-left max-w-3xl">
          <h2 className="text-4xl md:text-5xl font-medium tracking-tight font-switzer text-black dark:text-white leading-tight">
            Your next local AI stack is <br /> <HighlightText>one command away.</HighlightText>
          </h2>
        </div>
        
        {/* Right Side: Stacked Buttons */}
        <div className="flex flex-col gap-2.5 self-start md:self-center flex-shrink-0">
          <ShortcutButton text="Start" shortcutKey="S" href="/installation" variant="black" />
          <ShortcutButton text="Explore" shortcutKey="E" href="/registry" variant="white" />
        </div>
      </div>

      {/* FAQ Section */}
      <div className="w-full grid grid-cols-1 md:grid-cols-12 gap-8 text-left mt-44 px-2 self-start">
        {/* Left Column: Heading */}
        <div className="md:col-span-4">
          <h2 className="text-3xl md:text-4xl font-medium tracking-tight font-switzer text-black dark:text-white">
            Frequently Asked Questions
          </h2>
        </div>
        
        {/* Right Column: FAQ Accordion */}
        <div className="md:col-span-8 w-full">
          <FaqAccordion />
        </div>
      </div>

      {/* Third Box Container (Single-Section Box) */}
      <div className="relative w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] rounded-none mt-44 mb-24">
        
        {/* SVG Corner L-Brackets */}
        {/* Top-Left */}
        <svg 
          viewBox="0 0 12 12" 
          className="absolute -top-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white pointer-events-none"
        >
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        {/* Top-Right */}
        <svg 
          viewBox="0 0 12 12" 
          className="absolute -top-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none"
        >
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        {/* Bottom-Left */}
        <svg 
          viewBox="0 0 12 12" 
          className="absolute -bottom-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none"
        >
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>
        {/* Bottom-Right */}
        <svg 
          viewBox="0 0 12 12" 
          className="absolute -bottom-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none"
        >
          <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
        </svg>

        {/* Footer Content */}
        <div className="w-full py-12 px-6 md:px-12">
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-12 w-full">
            
            {/* Brand Section */}
            <div className="lg:col-span-2 flex flex-col items-start">
              <div className="scale-75 origin-left mb-6">
                <AsciiCanvas />
              </div>
              <p className="text-sm font-mono text-zinc-500 dark:text-zinc-400 leading-relaxed max-w-sm">
                The local infrastructure for collaborative team intelligence. Plug-and-play AI that stays in your office.
              </p>
            </div>

            {/* Links Columns */}
            <div className="lg:col-span-3 flex flex-wrap gap-x-12 gap-y-8 lg:justify-end w-full">
              {/* Product */}
              <div className="flex flex-col gap-4 min-w-[120px]">
                <h4 className="text-[11px] font-mono font-bold text-zinc-800 dark:text-zinc-200 uppercase tracking-[0.2em] mb-1">Product</h4>
                <ul className="flex flex-col gap-2.5">
                  <li>
                    <Link href="/registry" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      Registry
                    </Link>
                  </li>
                  <li>
                    <Link href="/feed" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      Feed
                    </Link>
                  </li>
                  <li>
                    <Link href="/installation" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      CLI Setup
                    </Link>
                  </li>
                </ul>
              </div>

              {/* Developers */}
              <div className="flex flex-col gap-4 min-w-[120px]">
                <h4 className="text-[11px] font-mono font-bold text-zinc-800 dark:text-zinc-200 uppercase tracking-[0.2em] mb-1">Developers</h4>
                <ul className="flex flex-col gap-2.5">
                  <li>
                    <Link href="/docs" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      API Docs
                    </Link>
                  </li>
                  <li>
                    <Link href="https://github.com/arnav080/Bloc" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      GitHub
                    </Link>
                  </li>
                  <li>
                    <Link href="/registry/submit" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      Submit Recipe
                    </Link>
                  </li>
                </ul>
              </div>

              {/* Repositories */}
              <div className="flex flex-col gap-4 min-w-[120px]">
                <h4 className="text-[11px] font-mono font-bold text-zinc-800 dark:text-zinc-200 uppercase tracking-[0.2em] mb-1">Repositories</h4>
                <ul className="flex flex-col gap-2.5">
                  <li>
                    <Link href="https://github.com/arnav080/Bloc/tree/main/cli" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      Go CLI Client
                    </Link>
                  </li>
                  <li>
                    <Link href="https://github.com/arnav080/Bloc/tree/main/hub" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      Next.js Hub
                    </Link>
                  </li>
                  <li>
                    <Link href="https://github.com/arnav080/Bloc/issues" className="text-[13px] font-mono text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors duration-300 border-b border-transparent hover:border-black/20 dark:hover:border-white/20 pb-0.5">
                      Feedback
                    </Link>
                  </li>
                </ul>
              </div>
            </div>

          </div>
          
          {/* Copyright section */}
          <div className="mt-12 pt-6 border-t border-zinc-200 dark:border-zinc-800/80 flex flex-col sm:flex-row items-center justify-between gap-4">
            <span className="text-[11px] font-mono text-zinc-400 dark:text-zinc-500">
              © {new Date().getFullYear()} Bloc AI Inc. All rights reserved.
            </span>
            <span className="text-[11px] font-mono text-zinc-400 dark:text-zinc-500 flex items-center gap-1.5">
              Designed for local intelligence <span className="w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse"></span>
            </span>
          </div>

        </div>
      </div>
    </div>
  );
}
