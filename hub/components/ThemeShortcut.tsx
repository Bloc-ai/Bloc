"use client";

import { useState, useEffect, useRef } from "react";
import { useTheme } from "next-themes";

export default function ThemeShortcut() {
  const { theme, resolvedTheme, setTheme } = useTheme();
  // H5 Fix: cache Audio instance in a ref — not re-created on every keypress
  const audioRef = useRef<HTMLAudioElement | null>(null);

  useEffect(() => {
    audioRef.current = new Audio("/images/switch-sound.mp3");
  }, []);

  // H8 Fix: use stable refs for theme values so the keydown listener
  // doesn't need to be re-registered on every theme change
  const resolvedThemeRef = useRef(resolvedTheme);
  const themeRef = useRef(theme);
  useEffect(() => { resolvedThemeRef.current = resolvedTheme; }, [resolvedTheme]);
  useEffect(() => { themeRef.current = theme; }, [theme]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Avoid triggering theme toggle when user is typing in form fields or inputs
      if (
        document.activeElement &&
        (document.activeElement.tagName === "INPUT" ||
          document.activeElement.tagName === "TEXTAREA" ||
          (document.activeElement as HTMLElement).isContentEditable)
      ) {
        return;
      }

      if (e.key.toLowerCase() === "m") {
        e.preventDefault();
        const currentTheme = resolvedThemeRef.current || themeRef.current;
        setTheme(currentTheme === "dark" ? "light" : "dark");
        // Play cached audio — reset to start so rapid keypresses replay cleanly
        if (audioRef.current) {
          audioRef.current.currentTime = 0;
          audioRef.current.play().catch(() => {});
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
    // Stable dep array — listener never re-registers on theme change
  }, [setTheme]);

  return null;
}
