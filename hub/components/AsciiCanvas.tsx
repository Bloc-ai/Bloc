"use client";

import React, { useEffect, useRef } from "react";

const ASCII_TEXT = `██████╗ ██╗      ██████╗  ██████╗
██╔══██╗██║     ██╔═══██╗██╔════╝
██████╔╝██║     ██║   ██║██║     
██╔══██╗██║     ██║   ██║██║     
██████╔╝███████╗╚██████╔╝╚██████╗
╚═════╝ ╚══════╝ ╚═════╝  ╚═════╝`;

export const AsciiCanvas = React.memo(() => {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const lines = ASCII_TEXT.split("\n");
    const charWidth = 10;
    const charHeight = 20;
    
    // Scale for high DPI
    const dpr = window.devicePixelRatio || 1;
    const logicalWidth = lines[0].length * charWidth;
    const logicalHeight = lines.length * charHeight;

    canvas.width = logicalWidth * dpr;
    canvas.height = logicalHeight * dpr;
    canvas.style.width = `${logicalWidth}px`;
    canvas.style.height = `${logicalHeight}px`;
    
    const draw = () => {
      if (!ctx) return;
      ctx.save();
      ctx.scale(dpr, dpr);
      
      // Clear and set background transparency
      ctx.clearRect(0, 0, logicalWidth, logicalHeight);
      
      // Set text style
      ctx.font = "18px 'Courier New', Courier, monospace";
      ctx.textBaseline = "top";
      ctx.textAlign = "left";

      // Detect dark mode strictly from document element class list
      const isDark = document.documentElement.classList.contains("dark");

      // Draw with a subtle glow in dark mode, flat black in light mode
      ctx.shadowColor = isDark ? "rgba(255, 255, 255, 0.4)" : "transparent";
      ctx.shadowBlur = isDark ? 10 : 0;
      ctx.fillStyle = isDark ? "#ffffff" : "#000000";

      lines.forEach((line, y) => {
        for (let x = 0; x < line.length; x++) {
          ctx.fillText(line[x], x * charWidth, y * charHeight);
        }
      });

      // Add a second pass for sharpness without glow
      ctx.shadowBlur = 0;
      lines.forEach((line, y) => {
        for (let x = 0; x < line.length; x++) {
          ctx.fillText(line[x], x * charWidth, y * charHeight);
        }
      });
      
      ctx.restore();
    };

    draw();

    // Listen to dark/light mode class changes on the html tag
    const observer = new MutationObserver(() => {
      draw();
    });
    
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    return () => {
      observer.disconnect();
    };
  }, []);

  return (
    <div className="relative flex items-center justify-center">
      <canvas 
        ref={canvasRef} 
        className="opacity-90 grayscale contrast-125 border-none outline-none block" 
        style={{ pointerEvents: 'none' }}
      />
    </div>
  );
});

AsciiCanvas.displayName = "AsciiCanvas";
