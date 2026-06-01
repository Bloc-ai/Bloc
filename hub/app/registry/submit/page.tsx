"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { supabase } from "@/lib/supabase";
import { toast } from "sonner";
import dynamic from "next/dynamic";
import { 
  ArrowLeft, 
  AlertCircle, 
  Download,
  BookOpen
} from "lucide-react";

// Monaco loaded lazily — avoids bundling ~2MB for unauthenticated page views
const Editor = dynamic(() => import("@monaco-editor/react"), {
  ssr: false,
  loading: () => (
    <div className="w-full flex items-center justify-center bg-[#1e1e1e] text-zinc-500 font-mono text-xs" style={{ height: 500 }}>
      Loading editor...
    </div>
  ),
});

// Minimal starter template for custom recipes
const DEFAULT_YAML_TEMPLATE = `# ──────────────────────────────────────────────────────────────────
#  BLOC RECIPE  ·  schema bloc/v1
#  Define your custom local AI execution environment parameters.
# ──────────────────────────────────────────────────────────────────

schema: "bloc/v1"
extends: null

metadata:
  name: "my-custom-recipe"
  description: "A short description of your optimized recipe configuration."
  tags: []
  author_notes: ""

model:
  source: "huggingface:username/model-repo"
  
  # For llama.cpp (GGUF):
  gguf_repo: "huggingface:username/model-repo-gguf"
  file: "model.gguf"
  download_url: "https://huggingface.co/username/model-repo-gguf/resolve/main/model.gguf"
  
  # For vLLM (Alternative to download_url/file):
  # hf_repo: "username/model-repo"
  
  quantization: "Q4_K_M"
  size_gb: 4.8
  parameters: "7B"
  architecture: "Dense"

engine:
  name: "llama.cpp"           # "llama.cpp" or "vllm"
  runtime: "native"           # "native" or "docker"
  version: "0.9.0"            # pinned engine version (semver format)
  tested_commit: "b5350"      # (llama.cpp only)

hardware:
  min_vram: "8GB"             # 4GB | 8GB | 12GB | 16GB | 24GB | Unified
  target_platform: "cuda"     # cuda | metal | rocm | cpu | vulkan
  gpu_count: 1
  recommended_ram: "16GB"

engine_config:
  host: "127.0.0.1"
  port: 8080
  n_parallel: 1

  # ─── llama.cpp-specific configurations ───
  ctx_size: 8192
  gpu_layers: 99
  flash_attn: true
  threads: 8
  jinja: true

  # ─── vLLM-specific configurations (uncomment if engine is vllm) ───
  # tensor_parallel_size: 1
  # gpu_memory_utilization: 0.90
  # max_model_len: 8192
  # trust_remote_code: false

  extra_args: []

pre_run:
  env: {}
  commands: []
`;

interface ParsedManifest {
  name: string;
  description: string;
  baseModel: string;
  engine: string;
  minVram: string;
  targetPlatform: string;
  version: string;
  isValid: boolean;
  error: string;
}

// Line-by-line YAML updater keeping precise layout and comments intact
const updateYamlField = (currentYaml: string, field: "name" | "min_vram" | "target_platform", newValue: string): string => {
  const lines = currentYaml.split("\n");
  let inMetadata = false;
  let inHardware = false;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();

    if (trimmed.startsWith("metadata:")) {
      inMetadata = true;
      inHardware = false;
      continue;
    }
    if (trimmed.startsWith("hardware:")) {
      inMetadata = false;
      inHardware = true;
      continue;
    }
    if (line.match(/^\S/) && !trimmed.startsWith("metadata:") && !trimmed.startsWith("hardware:")) {
      inMetadata = false;
      inHardware = false;
    }

    if (inMetadata && field === "name" && trimmed.startsWith("name:")) {
      const commentIdx = line.indexOf("#");
      const comment = commentIdx !== -1 ? line.substring(commentIdx) : "";
      const indent = line.match(/^\s*/)?.[0] || "  ";
      lines[i] = `${indent}name: "${newValue}"${comment ? " " + comment : ""}`;
      break;
    }

    if (inHardware) {
      if (field === "min_vram" && trimmed.startsWith("min_vram:")) {
        const commentIdx = line.indexOf("#");
        const comment = commentIdx !== -1 ? line.substring(commentIdx) : "";
        const indent = line.match(/^\s*/)?.[0] || "  ";
        lines[i] = `${indent}min_vram: "${newValue}"${comment ? " " + comment : ""}`;
        break;
      }
      if (field === "target_platform" && trimmed.startsWith("target_platform:")) {
        const commentIdx = line.indexOf("#");
        const comment = commentIdx !== -1 ? line.substring(commentIdx) : "";
        const indent = line.match(/^\s*/)?.[0] || "  ";
        lines[i] = `${indent}target_platform: "${newValue}"${comment ? " " + comment : ""}`;
        break;
      }
    }
  }

  return lines.join("\n");
};

export default function RecipeSubmitPage() {
  const router = useRouter();
  const { user, loading } = useAuth();

  // C1 Fix: Client-side auth guard (defence-in-depth — middleware handles server level)
  useEffect(() => {
    if (!loading && !user) {
      router.replace("/login?next=/registry/submit");
    }
  }, [user, loading, router]);

  const [yamlText, setYamlText] = useState(DEFAULT_YAML_TEMPLATE);
  const [parsed, setParsed] = useState<ParsedManifest>({
    name: "my-custom-recipe",
    description: "A short description of your optimized recipe configuration.",
    baseModel: "huggingface:username/model-repo",
    engine: "llama.cpp",
    minVram: "8GB",
    targetPlatform: "cuda",
    version: "bloc/v1",
    isValid: true,
    error: ""
  });

  const [isPublishing, setIsPublishing] = useState(false);
  const [editorHeight, setEditorHeight] = useState(500);

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    const startY = e.clientY;
    const startHeight = editorHeight;

    const handleMouseMove = (moveEvent: MouseEvent) => {
      const deltaY = moveEvent.clientY - startY;
      const newHeight = Math.max(500, startHeight + deltaY);
      setEditorHeight(newHeight);
    };

    const handleMouseUp = () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
  };

  // Parse YAML on text changes (with 200ms debounce)
  useEffect(() => {
    const handler = setTimeout(() => {
      const result: ParsedManifest = {
        name: "",
        description: "",
        baseModel: "",
        engine: "",
        minVram: "",
        targetPlatform: "",
        version: "",
        isValid: true,
        error: ""
      };

      try {
        const lines = yamlText.split("\n");
        let inMetadata = false;
        let inModel = false;
        let inEngine = false;   // new bloc/v1 engine: section
        let inHardware = false;

        for (let i = 0; i < lines.length; i++) {
          const line = lines[i];
          const trimmed = line.trim();

          // Mismatched or non-YAML key lines check
          if (trimmed && !trimmed.startsWith("#")) {
            if (!trimmed.includes(":") && !trimmed.startsWith("-") && !trimmed.startsWith("#") && !trimmed.startsWith("[")) {
              throw new Error(`Line ${i + 1}: Missing colon key-value pair.`);
            }
          }

          if (!trimmed || trimmed.startsWith("#")) continue;

          if (trimmed.startsWith("metadata:")) {
            inMetadata = true; inModel = false; inEngine = false; inHardware = false;
            continue;
          }
          if (trimmed.startsWith("model:")) {
            inMetadata = false; inModel = true; inEngine = false; inHardware = false;
            continue;
          }
          // bloc/v1: engine is its own top-level section
          if (trimmed.startsWith("engine:")) {
            inMetadata = false; inModel = false; inEngine = true; inHardware = false;
            continue;
          }
          if (trimmed.startsWith("hardware:")) {
            inMetadata = false; inModel = false; inEngine = false; inHardware = true;
            continue;
          }

          // Exit indented subsections on any non-indented line
          if (line.match(/^\S/)) {
            inMetadata = false; inModel = false; inEngine = false; inHardware = false;
          }

          const match = trimmed.match(/^([^:]+):\s*(.*)$/);
          if (match) {
            const key = match[1].trim();
            let val = match[2].trim();

            // Strip inline comments
            const hashIdx = val.indexOf("#");
            if (hashIdx !== -1) val = val.substring(0, hashIdx).trim();

            // Strip surrounding quotes
            if ((val.startsWith("\"") && val.endsWith("\"")) || (val.startsWith("'") && val.endsWith("'"))) {
              val = val.substring(1, val.length - 1);
            }

            if (inMetadata) {
              if (key === "name") result.name = val;
              if (key === "description") result.description = val;
            } else if (inModel) {
              // bloc/v1 uses model.source; legacy used model.base_model
              if (key === "source" || key === "base_model") result.baseModel = val;
            } else if (inEngine) {
              // bloc/v1: engine.name; legacy: model.engine
              if (key === "name") result.engine = val;
              if (key === "variant") result.version = val || "mainline";
            } else if (inHardware) {
              if (key === "min_vram") result.minVram = val;
              if (key === "target_platform") result.targetPlatform = val;
            } else {
              // Legacy schema: top-level version field
              if (key === "version" || key === "schema") result.version = val;
            }
          }
        }

        // Check required fields (support both bloc/v1 and legacy schema)
        if (!result.name) throw new Error("Missing required field: metadata.name");
        if (!result.baseModel) throw new Error("Missing required field: model.source (or model.base_model)");
        if (!result.minVram) throw new Error("Missing required field: hardware.min_vram");
        if (!result.targetPlatform) throw new Error("Missing required field: hardware.target_platform");

        setParsed({ ...result, isValid: true });
      } catch (err: any) {
        setParsed(prev => ({
          ...prev,
          isValid: false,
          error: err.message || "Invalid YAML manifest syntax."
        }));
      }
    }, 200);

    return () => clearTimeout(handler);
  }, [yamlText]);

  // Handle parameter field updates to instantly rewrite YAML string
  const handleFieldChange = (field: "name" | "min_vram" | "target_platform", val: string) => {
    const updatedYaml = updateYamlField(yamlText, field, val);
    setYamlText(updatedYaml);
  };



  // Download manifest file locally
  const handleDownloadYaml = () => {
    const blob = new Blob([yamlText], { type: "text/yaml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${parsed.name || "recipe"}.yaml`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast.success("Recipe YAML downloaded successfully!");
  };

  // Handle real publishing sequence in Supabase
  const handlePublish = async () => {
    if (!user) {
      toast.error("Authentication required.", {
        description: "Please sign in to publish optimization recipes directly."
      });
      return;
    }

    if (!parsed.isValid) {
      toast.error("Cannot publish invalid manifest.", {
        description: parsed.error
      });
      return;
    }

    if (!supabase) {
      toast.error("Database connection unavailable.", {
        description: "Supabase integration is not fully configured yet."
      });
      return;
    }

    setIsPublishing(true);
    const toastId = toast.loading("Uploading configuration to Supabase...");

    try {
      const { error } = await supabase
        .from("recipes")
        .insert({
          auth_id: user.id,
          creator: user.username,
          name: parsed.name,
          description: parsed.description,
          base_model: parsed.baseModel,
          min_vram: parsed.minVram,
          target_platform: parsed.targetPlatform,
          yaml_content: yamlText,
          tested_commit: yamlText.match(/tested_commit:\s*"([^"]+)"/)?.[1] || null
        });

      if (error) {
        if (error.code === '23505') {
          throw new Error(`You already have a recipe named "${parsed.name}".`);
        }
        throw error;
      }

      toast.dismiss(toastId);
      toast.success(`Successfully published ${user.username}/${parsed.name}!`, {
        description: "Your optimization recipe is now live on the registry."
      });
      router.push("/registry");
    } catch (err: any) {
      toast.dismiss(toastId);
      toast.error("Failed to publish recipe", {
        description: err.message || "An unexpected database error occurred."
      });
    } finally {
      setIsPublishing(false);
    }
  };

  return (
    <div className="max-w-5xl w-full mx-auto px-6 py-16 pt-24 min-h-screen relative">
      {/* Grid Background */}
      <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808005_1px,transparent_1px),linear-gradient(to_bottom,#80808005_1px,transparent_1px)] bg-[size:14px_24px] pointer-events-none -z-10" />

      {/* Breadcrumbs Header */}
      <div className="mb-10 text-left border-b border-zinc-200 dark:border-zinc-800 pb-8 flex flex-col md:flex-row md:items-end md:justify-between gap-6">
        <div>
          <Link href="/registry" className="inline-flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors mb-4 group">
            <ArrowLeft className="w-3 h-3 transition-transform group-hover:-translate-x-0.5" />
            Back to registry
          </Link>
          <h1 className="text-4xl font-semibold tracking-tight font-switzer text-black dark:text-white leading-none">
            Publish Optimization Recipe
          </h1>
          <p className="text-zinc-500 dark:text-zinc-400 font-switzer font-medium text-sm md:text-base max-w-2xl leading-relaxed mt-3.5">
            Upload custom local configurations, engine quantization specs, and memory parameters to power community model configurations.
          </p>
        </div>

        <div className="flex flex-col gap-2 shrink-0 w-full md:w-44">
          <Link
            href="/docs/recipes"
            target="_blank"
            className="flex items-center justify-center gap-2 h-9 w-full border border-dashed border-zinc-400 dark:border-zinc-700 bg-zinc-100 hover:bg-zinc-200 dark:bg-zinc-900/40 dark:hover:bg-zinc-900/80 font-mono text-[9px] uppercase font-bold tracking-wider cursor-pointer text-zinc-600 hover:text-black dark:text-zinc-400 dark:hover:text-zinc-250 transition-colors rounded-none"
          >
            <BookOpen className="w-3.5 h-3.5" />
            YAML Schema Docs
          </Link>

          <button
            onClick={handleDownloadYaml}
            className="flex items-center justify-center gap-2 h-9 w-full border border-black dark:border-white font-mono text-[10px] uppercase font-bold tracking-wider cursor-pointer bg-transparent transition-colors text-black dark:text-white hover:opacity-80"
          >
            <Download className="w-3.5 h-3.5" />
            Download YAML
          </button>
          
          <button
            onClick={handlePublish}
            disabled={isPublishing}
            className="group relative flex items-center justify-center h-9 w-full bg-blue-500 hover:bg-blue-600 text-white font-mono text-[10px] uppercase font-bold tracking-wider transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isPublishing ? "Publishing..." : "Publish Recipe"}
          </button>
        </div>
      </div>


      {/* Editor Main Content Area */}
      <div className="flex flex-col gap-6">
        
        {/* Dynamic Image 2 style recipe path preview header */}
        <div className="flex flex-col gap-3">
          <div className="text-3xl font-switzer font-semibold text-black dark:text-white flex items-center select-text">
            <span className="leading-none shrink-0">
              {user?.username || "anonymous"}
            </span>
            <span className="mx-1.5 text-zinc-450 dark:text-zinc-600 leading-none shrink-0">/</span>
            <span className="text-blue-600 dark:text-blue-400 leading-none truncate" title={parsed.name || "recipe"}>
              {parsed.name || "recipe"}
            </span>
          </div>

          {/* Dynamic Syntax Error warning block if YAML fails parsing */}
          {!parsed.isValid && (
            <div className="p-4 border border-red-500/25 bg-red-500/5 text-red-600 dark:text-red-450 relative select-none">
              <div className="flex gap-3">
                <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
                <div>
                  <h3 className="font-switzer font-bold text-xs uppercase tracking-wider">Syntax Validation Failed</h3>
                  <p className="font-mono text-[10px] mt-1 whitespace-pre-wrap leading-relaxed opacity-95">
                    {parsed.error}
                  </p>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Brutalist Parameter Input Grid (Synced bi-directionally) */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 p-6 border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 relative">
          {/* L-Brackets */}
          <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white pointer-events-none">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] pointer-events-none">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-y-[-1] pointer-events-none">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-2.5 h-2.5 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>

          {/* Recipe Name Input */}
          <div className="flex flex-col gap-1.5">
            <label className="font-mono text-[9px] text-zinc-450 dark:text-zinc-500 font-bold uppercase tracking-wider">
              Recipe Name
            </label>
            <input
              type="text"
              value={parsed.name || ""}
              onChange={(e) => handleFieldChange("name", e.target.value)}
              className="bg-[#1e1e1e] border border-zinc-300 dark:border-zinc-800 focus:border-blue-500 outline-none font-mono text-xs text-white px-3 h-10 w-full rounded-none"
              placeholder="e.g. qwen-7b-budget-beast"
            />
          </div>

          {/* Platform Acceleration Dropdown */}
          <div className="flex flex-col gap-1.5">
            <label className="font-mono text-[9px] text-zinc-450 dark:text-zinc-500 font-bold uppercase tracking-wider">
              Platform Acceleration
            </label>
            <select
              value={parsed.targetPlatform || "cuda"}
              onChange={(e) => handleFieldChange("target_platform", e.target.value)}
              className="bg-[#1e1e1e] border border-zinc-300 dark:border-zinc-800 focus:border-blue-500 outline-none font-mono text-xs text-white px-3 h-10 w-full rounded-none cursor-pointer"
            >
              <option value="cuda">CUDA (Nvidia)</option>
              <option value="metal">METAL (Apple Silicon)</option>
              <option value="rocm">ROCM (AMD)</option>
              <option value="cpu">CPU (No Acceleration)</option>
            </select>
          </div>

          {/* Memory VRAM Input Textbox */}
          <div className="flex flex-col gap-1.5">
            <label className="font-mono text-[9px] text-zinc-450 dark:text-zinc-500 font-bold uppercase tracking-wider">
              Memory Constraint (VRAM)
            </label>
            <input
              type="text"
              value={parsed.minVram || ""}
              onChange={(e) => handleFieldChange("min_vram", e.target.value)}
              className="bg-[#1e1e1e] border border-zinc-300 dark:border-zinc-800 focus:border-blue-500 outline-none font-mono text-xs text-white px-3 h-10 w-full rounded-none"
              placeholder="e.g. 16GB"
            />
          </div>
        </div>

        {/* YAML Code Editor Container */}
        <div className="flex flex-col border border-zinc-300 dark:border-zinc-800 bg-[#1e1e1e] relative">
          
          {/* SVG L-Brackets */}
          <svg viewBox="0 0 12 12" className="absolute -top-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white pointer-events-none z-10">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -top-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none z-10">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -left-[1px] w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none z-10">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>
          <svg viewBox="0 0 12 12" className="absolute -bottom-[1px] -right-[1px] w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none z-10">
            <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
          </svg>

          {/* Monaco Editor Canvas */}
          <div 
            style={{ height: `${editorHeight}px` }} 
            className="overflow-hidden bg-[#1e1e1e] relative w-full"
          >
            <Editor
              height={`${editorHeight}px`}
              defaultLanguage="yaml"
              theme="vs-dark"
              value={yamlText}
              onChange={(val) => setYamlText(val || "")}
              loading={
                <div className="flex items-center justify-center h-full text-zinc-450 font-mono text-[10px]">
                  Loading Monaco Workspace Canvas...
                </div>
              }
              options={{
                minimap: { enabled: false },
                fontSize: 12,
                lineHeight: 20,
                fontFamily: "JetBrains Mono, Fira Code, Menlo, Monaco, monospace",
                wordWrap: "on",
                lineNumbersMinChars: 3,
                scrollBeyondLastLine: false,
                automaticLayout: true,
                padding: { top: 12, bottom: 12 },
                renderLineHighlight: "all",
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

      </div>
    </div>
  );
}
