import { Metadata } from "next";
import { Activity, Terminal, Rocket, Shield, GitCommit, LucideIcon } from "lucide-react";
import fs from "fs";
import path from "path";

export const metadata: Metadata = {
  title: "Updates - Bloc",
  description: "Changelog and release notes for the Bloc AI platform.",
};

type BulletPoint = {
  text: string;
  depth: number;
};

type UpdateItem = {
  version: string;
  date: string;
  title: string;
  points: BulletPoint[];
  icon: LucideIcon;
};

function parseChangelog(): UpdateItem[] {
  let fileContent = "";
  try {
    fileContent = fs.readFileSync(path.join(process.cwd(), "..", "CHANGELOG.md"), "utf-8");
  } catch (e) {
    try {
      fileContent = fs.readFileSync(path.join(process.cwd(), "CHANGELOG.md"), "utf-8");
    } catch (err) {
      console.error("Could not read CHANGELOG.md");
      return [];
    }
  }

  const updates: UpdateItem[] = [];
  const blocks = fileContent.split(/^#\s+/m).filter((block) => block.trim().length > 0);

  for (const block of blocks) {
    const lines = block.split("\n").filter((l) => l.trim().length > 0);
    if (lines.length < 2) continue;

    // Parse the heading: v0.5.2 (June 2026)
    const headingMatch = lines[0].match(/([^\s]+)\s+\((.+?)\)/);
    let version = lines[0].trim();
    let date = "";
    if (headingMatch) {
      version = headingMatch[1];
      date = headingMatch[2];
    }

    // First bullet is title
    let title = "";
    const points: BulletPoint[] = [];

    // Parse lines to extract title and points with hierarchy
    let firstLine = lines[1];
    if (firstLine.trim().startsWith("- ")) {
      title = firstLine.trim().substring(2).trim();
    } else {
      title = firstLine.trim();
    }

    for (let i = 2; i < lines.length; i++) {
      const originalLine = lines[i];
      const trimmedLine = originalLine.trim();
      
      // Determine indentation depth
      const leadingSpaces = originalLine.match(/^\s*/)?.[0].length || 0;
      const depth = leadingSpaces >= 2 ? 1 : 0;

      let text = trimmedLine;
      if (text.startsWith("- ")) {
        text = text.substring(2).trim();
      }

      points.push({ text, depth });
    }
    
    // Choose icon
    let icon = Activity;
    if (version.endsWith(".0")) {
      icon = Rocket;
    } else if (title.toLowerCase().includes("security") || title.toLowerCase().includes("hardening") || title.toLowerCase().includes("fix")) {
      icon = Shield;
    } else if (title.toLowerCase().includes("terminal") || title.toLowerCase().includes("tui") || title.toLowerCase().includes("tab")) {
      icon = Terminal;
    }

    updates.push({
      version,
      date,
      title,
      points,
      icon,
    });
  }

  return updates;
}

function renderFormattedText(text: string) {
  const regex = /(\*\*.*?\*\*|`.*?`)/g;
  const parts = text.split(regex);

  return parts.map((part, index) => {
    if (part.startsWith("**") && part.endsWith("**")) {
      return (
        <strong key={index} className="font-bold text-black dark:text-white">
          {part.slice(2, -2)}
        </strong>
      );
    } else if (part.startsWith("`") && part.endsWith("`")) {
      return (
        <code key={index} className="font-mono bg-black/5 dark:bg-white/10 px-1.5 py-0.5 rounded text-xs text-black dark:text-white border border-black/10 dark:border-white/10 mx-0.5">
          {part.slice(1, -1)}
        </code>
      );
    }
    return part;
  });
}

export default function UpdatesPage() {
  const updates = parseChangelog();

  return (
    <div className="min-h-screen pt-24 pb-20 px-4 md:px-8 max-w-4xl mx-auto">
      <div className="mb-16">
        <h1 className="text-4xl md:text-5xl font-medium tracking-tight font-switzer text-black dark:text-white mb-4">
          Changelog
        </h1>
        <p className="text-zinc-500 dark:text-zinc-400 font-mono text-sm max-w-xl">
          Track our rapid iteration. We ship updates constantly to bring you the best local AI developer experience.
        </p>
      </div>

      <div className="relative border-l border-black/10 dark:border-white/10 ml-4 md:ml-6 space-y-24 pb-8">
        {updates.map((update, i) => {
          const Icon = update.icon;
          const isMajor = update.version.endsWith(".0");
          const isPatch = !isMajor;
          
          return (
            <div key={update.version} className="relative pl-10 md:pl-16">
              {/* Timeline Marker - Square instead of circle for brutalist look */}
              <div className="absolute -left-[17px] top-6 w-8 h-8 rounded-md bg-[#f6f6f3] dark:bg-[#121212] border border-black/20 dark:border-white/20 flex items-center justify-center text-black/60 dark:text-white/60 z-10 shadow-sm">
                <Icon className="w-3.5 h-3.5" />
              </div>

              {/* Content Box */}
              <div className={`relative border border-black/10 dark:border-white/10 ${isMajor ? "bg-black/5 dark:bg-white/5" : "bg-transparent"} p-6 md:p-8 rounded-lg`}>
                
                {/* Brutalist Corners for Major Releases */}
                {isMajor && (
                  <>
                    <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2.5 h-2.5 fill-black/40 dark:fill-white/40 pointer-events-none"><path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" /></svg>
                    <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2.5 h-2.5 fill-black/40 dark:fill-white/40 scale-x-[-1] pointer-events-none"><path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" /></svg>
                    <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2.5 h-2.5 fill-black/40 dark:fill-white/40 scale-y-[-1] pointer-events-none"><path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" /></svg>
                    <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2.5 h-2.5 fill-black/40 dark:fill-white/40 scale-x-[-1] scale-y-[-1] pointer-events-none"><path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" /></svg>
                  </>
                )}

                <div className="flex flex-col md:flex-row md:items-baseline gap-3 md:gap-4 mb-4">
                  <h2 className={`tracking-tight font-switzer text-black dark:text-white ${
                    isPatch ? "text-lg md:text-xl font-semibold" : "text-xl md:text-2xl font-bold"
                  }`}>
                    {renderFormattedText(update.title)}
                  </h2>
                  <div className="flex items-center gap-3 font-mono text-[10px] md:text-xs uppercase tracking-wider">
                    <span className={`px-2 py-1 font-bold ${
                      isMajor
                        ? "bg-[#2563EB] text-white rounded-md shadow-sm"
                        : "bg-black/5 dark:bg-white/10 text-black/70 dark:text-white/80 rounded-md border border-black/10 dark:border-white/10"
                    }`}>
                      {update.version}
                    </span>
                    <span className="text-zinc-500 flex items-center gap-2">
                      <span className="w-1 h-1 rounded-full bg-zinc-300 dark:bg-zinc-700"></span>
                      {update.date}
                    </span>
                  </div>
                </div>
                
                {update.points.length > 0 && (
                  <ul className={`space-y-2.5 font-mono ${
                    isPatch ? "text-[12px] md:text-xs" : "text-[13px] md:text-sm"
                  } text-zinc-600 dark:text-zinc-350 max-w-2xl mt-4`}>
                    {update.points.map((pt, idx) => {
                      if (pt.depth === 0) {
                        return (
                          <li key={idx} className="font-semibold text-black dark:text-white mt-4 first:mt-0 list-none flex items-start gap-2">
                            <span className="text-[#2563EB] shrink-0 mt-1.5 text-[8px]">●</span>
                            <span>{renderFormattedText(pt.text)}</span>
                          </li>
                        );
                      } else {
                        return (
                          <li key={idx} className="pl-5 list-none flex items-start gap-2 text-zinc-600 dark:text-zinc-400">
                            <span className="text-zinc-400 dark:text-zinc-650 shrink-0 mt-1.5 text-[6px]">■</span>
                            <span>{renderFormattedText(pt.text)}</span>
                          </li>
                        );
                      }
                    })}
                  </ul>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
