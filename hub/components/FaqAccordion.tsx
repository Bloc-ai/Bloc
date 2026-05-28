"use client";

import React, { useState } from "react";

interface FaqItem {
  question: string;
  answer: string;
}

export default function FaqAccordion() {
  const [openIndex, setOpenIndex] = useState<number | null>(null);

  const faqs: FaqItem[] = [
    {
      question: "What is a local AI recipe?",
      answer: "A recipe is a pre-configured, reproducible environment descriptor that defines the model weight quantization, llama.cpp startup parameters, system prompts, and required hardware profile (CPU, Metal, or specific CUDA setups) to run an LLM optimally.",
    },
    {
      question: "How do I deploy a recipe?",
      answer: "Install the Bloc CLI using npm or brew, then run 'bloc deploy <recipe-name>'. The CLI pulls the optimized weights and handles dependencies, starting a local API server in seconds.",
    },
    {
      question: "Is Bloc Hub fully open-source and local?",
      answer: "Yes. Bloc Hub is built entirely on open infrastructure. All models run completely on your own machine without sending data to external APIs, ensuring absolute privacy.",
    },
    {
      question: "What hardware is supported?",
      answer: "We support Apple Silicon (Metal acceleration), Nvidia GPUs (CUDA), AMD GPUs (ROCm), and standard CPU-only configurations. The registry automatically matches recipes to your hardware.",
    },
    {
      question: "How can I share my own recipe?",
      answer: "Simply define your hardware targets and run parameters in a 'bloc.yaml' file and use the 'bloc publish' command to share it with the registry.",
    },
  ];

  return (
    <div className="w-full flex flex-col gap-4 font-switzer select-none">
      {faqs.map((item, index) => {
        const isOpen = openIndex === index;

        return (
          <div
            key={index}
            className={`relative w-full border rounded-none transition-all duration-300 ease-in-out ${
              isOpen
                ? "border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] shadow-xs"
                : "border-transparent bg-transparent border-b border-b-zinc-300 dark:border-b-zinc-800/80"
            }`}
          >
            {/* Corner L-brackets (Fade and scale in when open) */}
            <svg
              viewBox="0 0 12 12"
              className={`absolute -top-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white pointer-events-none transition-all duration-300 ease-in-out ${
                isOpen ? "opacity-100 scale-100" : "opacity-0 scale-90"
              }`}
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg
              viewBox="0 0 12 12"
              className={`absolute -top-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-300 ease-in-out ${
                isOpen ? "opacity-100 scale-100" : "opacity-0 scale-90"
              }`}
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg
              viewBox="0 0 12 12"
              className={`absolute -bottom-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-300 ease-in-out ${
                isOpen ? "opacity-100 scale-100" : "opacity-0 scale-90"
              }`}
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>
            <svg
              viewBox="0 0 12 12"
              className={`absolute -bottom-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-300 ease-in-out ${
                isOpen ? "opacity-100 scale-100" : "opacity-0 scale-90"
              }`}
            >
              <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
            </svg>

            {/* Section 1: Question (Always visible header) */}
            <div
              className={`w-full flex items-center justify-between py-4 px-4 md:px-6 cursor-pointer select-none transition-colors duration-200 ${
                isOpen
                  ? "text-black dark:text-white"
                  : "text-zinc-800 dark:text-zinc-200 hover:text-[#2563EB] dark:hover:text-[#2563EB]"
              }`}
              onClick={() => setOpenIndex(isOpen ? null : index)}
            >
              <span className="font-switzer font-medium text-base md:text-lg">
                {item.question}
              </span>
              <span
                className={`text-xl text-zinc-400 font-light flex items-center justify-center w-6 h-6 transition-all duration-300 ${
                  isOpen ? "rotate-45 text-zinc-800 dark:text-zinc-200 scale-110" : "rotate-0"
                }`}
              >
                &#43;
              </span>
            </div>

            {/* Section 2: Answer (Collapsible with smooth grid template rows height transition) */}
            <div
              className="grid transition-all duration-300 ease-in-out"
              style={{
                gridTemplateRows: isOpen ? "1fr" : "0fr",
                opacity: isOpen ? 1 : 0,
              }}
            >
              <div className="overflow-hidden">
                {/* Horizontal Divider with T-brackets */}
                <div
                  className={`border-t border-zinc-300 dark:border-zinc-800 relative transition-opacity duration-300 ${
                    isOpen ? "opacity-100" : "opacity-0"
                  }`}
                >
                  <svg
                    viewBox="0 0 12 16"
                    className="absolute -left-[1px] top-0 -translate-y-1/2 w-3 h-4 fill-black dark:fill-white pointer-events-none"
                  >
                    <path d="M 0,0 L 1,0 L 1,4 Q 1,7 4,7 L 12,7 L 12,8 L 4,8 Q 1,8 1,11 L 1,16 L 0,16 Z" />
                  </svg>
                  <svg
                    viewBox="0 0 12 16"
                    className="absolute -right-[1px] top-0 -translate-y-1/2 w-3 h-4 fill-black dark:fill-white scale-x-[-1] pointer-events-none"
                  >
                    <path d="M 0,0 L 1,0 L 1,4 Q 1,7 4,7 L 12,7 L 12,8 L 4,8 Q 1,8 1,11 L 1,16 L 0,16 Z" />
                  </svg>
                </div>

                <div className="w-full py-4 px-6 text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base leading-relaxed bg-[#fbfbfa] dark:bg-[#1a1a1a]">
                  {item.answer}
                </div>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
