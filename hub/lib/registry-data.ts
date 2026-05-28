export interface Benchmark {
  gpu: string;
  tokensPerSec: number;
  runs: number;
}

export interface Recipe {
  id: string; // e.g., 'alice/qwen-7b-budget-beast'
  name: string;
  creator: string;
  description: string;
  baseModel: string; // e.g., 'huggingface:Qwen/Qwen2.5-7B-Instruct'
  engine: string; // e.g., 'llama.cpp'
  quantization: string; // e.g., 'Q4_K_M'
  hardware: {
    minVram: string; // '4GB' | '8GB' | '12GB' | '16GB' | '24GB' | 'Unified'
    targetPlatform: 'cuda' | 'metal' | 'rocm' | 'cpu';
  };
  verified: 'publisher' | 'community' | 'none';
  telemetry: {
    runs: number;
    benchmarks: Benchmark[];
  };
}

export const registryRecipes: Recipe[] = [];
