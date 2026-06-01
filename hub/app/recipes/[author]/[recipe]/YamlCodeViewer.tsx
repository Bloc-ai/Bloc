"use client";

import { useState } from "react";
import { Copy, Check, FileCode, Edit2, Save, X } from "lucide-react";
import { toast } from "sonner";
import { useAuth } from "@/context/AuthContext";
import { supabase } from "@/lib/supabase";
import { useRouter } from "next/navigation";
import dynamic from "next/dynamic";

// Monaco loaded lazily — avoids bundling ~2MB for unauthenticated page views
const Editor = dynamic(() => import("@monaco-editor/react"), {
  ssr: false,
  loading: () => (
    <div className="w-full flex items-center justify-center bg-[#1e1e1e] text-zinc-500 font-mono text-xs" style={{ height: 480 }}>
      Loading workspace editor...
    </div>
  ),
});

interface YamlCodeViewerProps {
  yaml: string;
  filename: string;
  creator: string;
  isMock?: boolean;
}

export default function YamlCodeViewer({ 
  yaml, 
  filename,
  creator,
  isMock = false
}: YamlCodeViewerProps) {
  const { user } = useAuth();
  const router = useRouter();
  
  const [copied, setCopied] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [code, setCode] = useState(yaml);
  const [isSaving, setIsSaving] = useState(false);
  const linesCount = yaml.split("\n").length;
  const initialHeight = Math.max(300, (linesCount * 20) + 24);
  const [editorHeight, setEditorHeight] = useState(initialHeight);

  const canEdit = user && user.username === creator && !isMock;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(isEditing ? code : yaml);
      setCopied(true);
      toast.success("YAML copied to clipboard!");
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error("Failed to copy YAML: ", err);
    }
  };

  const handleCancel = () => {
    setCode(yaml);
    setIsEditing(false);
  };

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    const startY = e.clientY;
    const startHeight = editorHeight;

    const handleMouseMove = (moveEvent: MouseEvent) => {
      const deltaY = moveEvent.clientY - startY;
      const newHeight = Math.max(300, startHeight + deltaY);
      setEditorHeight(newHeight);
    };

    const handleMouseUp = () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
  };

  const handleSave = async () => {
    if (isMock) return;

    setIsSaving(true);
    const toastId = toast.loading("Saving changes to registry...");

    try {
      // Parse YAML to extract metadata fields for synchronization
      const result = {
        description: "",
        baseModel: "",
        minVram: "",
        targetPlatform: "",
      };

      const lines = code.split("\n");
      let inMetadata = false;
      let inModel = false;
      let inHardware = false;

      for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const trimmed = line.trim();

        if (!trimmed || trimmed.startsWith("#")) continue;

        if (trimmed.startsWith("metadata:")) {
          inMetadata = true; inModel = false; inHardware = false;
          continue;
        }
        if (trimmed.startsWith("model:")) {
          inMetadata = false; inModel = true; inHardware = false;
          continue;
        }
        if (trimmed.startsWith("hardware:")) {
          inMetadata = false; inModel = false; inHardware = true;
          continue;
        }

        if (line.match(/^\S/)) {
          inMetadata = false; inModel = false; inHardware = false;
        }

        const match = trimmed.match(/^([^:]+):\s*(.*)$/);
        if (match) {
          const key = match[1].trim();
          let val = match[2].trim();

          const hashIdx = val.indexOf("#");
          if (hashIdx !== -1) val = val.substring(0, hashIdx).trim();

          if ((val.startsWith("\"") && val.endsWith("\"")) || (val.startsWith("'") && val.endsWith("'"))) {
            val = val.substring(1, val.length - 1);
          }

          if (inMetadata) {
            if (key === "description") result.description = val;
          } else if (inModel) {
            if (key === "source" || key === "base_model") result.baseModel = val;
          } else if (inHardware) {
            if (key === "min_vram") result.minVram = val;
            if (key === "target_platform") result.targetPlatform = val;
          }
        }
      }

      if (!supabase) {
        throw new Error("Database connection unavailable.");
      }

      const recipeName = filename.replace(".yaml", "");
      const { error } = await supabase
        .from("recipes")
        .update({
          yaml_content: code,
          description: result.description || null,
          base_model: result.baseModel || null,
          min_vram: result.minVram || null,
          target_platform: result.targetPlatform || null,
          tested_commit: code.match(/tested_commit:\s*"([^"]+)"/)?.[1] || null
        })
        .eq("creator", creator)
        .eq("name", recipeName);

      if (error) throw error;

      toast.dismiss(toastId);
      toast.success("Recipe updated successfully!");
      setIsEditing(false);
      
      // Refresh page data
      router.refresh();
    } catch (err: any) {
      toast.dismiss(toastId);
      toast.error("Failed to save changes", {
        description: err.message || "An unexpected error occurred."
      });
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="mt-16 w-full flex flex-col font-mono text-xs text-zinc-300 bg-[#1e1e1e] border border-zinc-300 dark:border-zinc-800 rounded-none relative overflow-hidden shadow-2xl">
      {/* SVG Corner L-Brackets */}
      <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white pointer-events-none z-10">
        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
      </svg>
      <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] pointer-events-none z-10">
        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
      </svg>
      <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-y-[-1] pointer-events-none z-10">
        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
      </svg>
      <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none z-10">
        <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
      </svg>

      {/* Header Bar */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-zinc-250 dark:border-zinc-900 bg-zinc-200 dark:bg-zinc-950 select-none">
        <div className="flex items-center gap-2">
          <FileCode className="w-3.5 h-3.5 text-zinc-500" />
          <span className="text-[10px] text-zinc-700 dark:text-zinc-450 font-bold uppercase tracking-wider">{filename}</span>
        </div>
        <div className="flex items-center gap-2">
          {canEdit && (
            isEditing ? (
              <>
                <button
                  onClick={handleCancel}
                  disabled={isSaving}
                  className="flex items-center gap-1.5 px-2.5 py-1 bg-zinc-100 hover:bg-zinc-250 dark:bg-zinc-800 dark:hover:bg-zinc-700 border border-zinc-300 dark:border-zinc-700 text-zinc-800 hover:text-black dark:text-white transition-all duration-150 rounded-md text-[9px] font-mono uppercase font-bold tracking-wider cursor-pointer disabled:opacity-50"
                >
                  <X className="w-3.5 h-3.5 text-red-500" />
                  <span>Cancel</span>
                </button>
                <button
                  onClick={handleSave}
                  disabled={isSaving}
                  className="flex items-center gap-1.5 px-2.5 py-1 bg-emerald-600 hover:bg-emerald-700 border border-transparent text-white transition-all duration-150 rounded-md text-[9px] font-mono uppercase font-bold tracking-wider cursor-pointer disabled:opacity-50 shadow-sm"
                >
                  <Save className="w-3.5 h-3.5" />
                  <span>{isSaving ? "Saving..." : "Save YAML"}</span>
                </button>
              </>
            ) : (
              <button
                onClick={() => setIsEditing(true)}
                className="flex items-center gap-1.5 px-2.5 py-1 bg-blue-500 hover:bg-blue-600 border border-transparent text-white transition-all duration-150 rounded-md text-[9px] font-mono uppercase font-bold tracking-wider cursor-pointer shadow-sm"
              >
                <Edit2 className="w-3.5 h-3.5" />
                <span>Edit YAML</span>
              </button>
            )
          )}
          <button
            onClick={handleCopy}
            className="flex items-center gap-1.5 px-2.5 py-1 bg-zinc-100 hover:bg-zinc-250 dark:bg-zinc-800 dark:hover:bg-zinc-700 border border-zinc-300 dark:border-zinc-700 text-zinc-800 hover:text-black dark:text-white transition-all duration-150 rounded-md text-[9px] font-mono uppercase font-bold tracking-wider cursor-pointer"
          >
            {copied ? (
              <>
                <Check className="w-3.5 h-3.5 text-emerald-500" />
                <span>Copied</span>
              </>
            ) : (
              <>
                <Copy className="w-3.5 h-3.5" />
                <span>Copy YAML</span>
              </>
            )}
          </button>
        </div>
      </div>

      {/* Code Editor Area (Monaco) */}
      <div className="overflow-hidden bg-[#1e1e1e] relative w-full border-t border-[#2d2d2d]" style={{ height: editorHeight }}>
        <Editor
          height={`${editorHeight}px`}
          defaultLanguage="yaml"
          theme="vs-dark"
          value={isEditing ? code : yaml}
          onChange={(val) => {
            if (isEditing) setCode(val || "");
          }}
          options={{
            readOnly: !isEditing,
            minimap: { enabled: false },
            fontSize: 12,
            lineHeight: 20,
            fontFamily: "JetBrains Mono, Fira Code, Menlo, Monaco, monospace",
            wordWrap: "on",
            lineNumbersMinChars: 3,
            scrollBeyondLastLine: false,
            automaticLayout: true,
            padding: { top: 12, bottom: 12 },
            renderLineHighlight: isEditing ? "all" : "none",
            bracketPairColorization: { enabled: true },
            folding: true,
            glyphMargin: false,
            foldingHighlight: true,
            scrollbar: {
              vertical: "visible",
              horizontal: "auto",
              verticalScrollbarSize: 8,
              horizontalScrollbarSize: 8
            }
          }}
        />
      </div>

      {/* Drag Resizer Handle */}
      <div 
        onMouseDown={handleMouseDown}
        className="h-2 w-full bg-[#1e1e1e] hover:bg-zinc-800 border-t border-[#2d2d2d] cursor-ns-resize flex items-center justify-center transition-colors group select-none relative z-20"
      >
        <div className="w-8 h-1 rounded-full bg-zinc-700 group-hover:bg-blue-500 transition-colors" />
      </div>
    </div>
  );
}
