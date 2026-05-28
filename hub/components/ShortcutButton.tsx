"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

interface ShortcutButtonProps {
  text: string;
  shortcutKey: string; // e.g., 'S', 'D'
  href?: string;
  onClick?: () => void;
  variant?: "black" | "white" | "primary";
  className?: string;
}

export default function ShortcutButton({
  text,
  shortcutKey,
  href,
  onClick,
  variant = "black",
  className = "",
}: ShortcutButtonProps) {
  const router = useRouter();
  const [isPressed, setIsPressed] = useState(false);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Avoid triggering shortcut when user is typing in form inputs
      if (
        document.activeElement &&
        (document.activeElement.tagName === "INPUT" ||
          document.activeElement.tagName === "TEXTAREA" ||
          (document.activeElement as HTMLElement).isContentEditable)
      ) {
        return;
      }

      if (e.key.toLowerCase() === shortcutKey.toLowerCase()) {
        e.preventDefault();
        triggerClick();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [shortcutKey, href, onClick]);

  const triggerClick = () => {
    setIsPressed(true);
    
    // Brief delay to animate the "pressed" state before executing action
    setTimeout(() => {
      setIsPressed(false);
      if (onClick) {
        onClick();
      } else if (href) {
        if (href.startsWith("http")) {
          window.location.href = href;
        } else {
          router.push(href);
        }
      }
    }, 150);
  };

  // Base styling and color profiles matching the user's design reference (no rounded corners, tighter padding, relative group for hover brackets)
  const baseStyles = "group relative inline-flex items-center justify-center gap-2 font-switzer font-medium text-sm rounded-none border transition-all duration-150 cursor-pointer select-none px-2.5 py-1";
  
  // Active/pressed scale and shadow overrides
  const pressedStyles = isPressed ? "scale-[0.96] brightness-90" : "hover:scale-[1.01] hover:brightness-105 active:scale-[0.96]";

  const variantStyles = {
    black: "bg-[#1e1e1d] hover:bg-[#2c2c2b] text-white border-[#2c2c2b]",
    white: "bg-[#f6f6f3] hover:bg-[#ededeb] text-[#171616] border-[#c4c4be]",
    primary: "bg-[#2563EB] hover:bg-[#1d4ed8] text-white border-[#1d4ed8]"
  };

  const badgeStyles = {
    black: "bg-[#2c2c2b] text-zinc-400 border-[rgba(255,255,255,0.08)]",
    white: "bg-[#e5e5e0] text-[#171616] border-[#c4c4be]",
    primary: "bg-[#1d4ed8] text-blue-200 border-[rgba(255,255,255,0.15)]"
  };

  return (
    <button
      onClick={triggerClick}
      className={`${baseStyles} ${pressedStyles} ${variantStyles[variant]} ${className}`}
    >
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

      <span>{text}</span>
      <span className={`inline-flex items-center justify-center w-[18px] h-[18px] text-[10px] font-switzer font-medium rounded-none border uppercase ${badgeStyles[variant]}`}>
        {shortcutKey}
      </span>
    </button>
  );
}
