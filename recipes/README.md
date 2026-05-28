# Bloc Recipes Registry

This directory contains the community-curated configurations (recipes) for running local AI models with `llama.cpp` using the Bloc CLI.

To submit a new recipe, follow the GitOps flow outlined below.

---

## Folder Structure

All recipes are organized by the author's GitHub/Bloc username to prevent name conflicts:

```
/recipes
  ├── README.md               # This file
  ├── TEMPLATE.yaml           # Starting template
  └── [your-username]/        # Your namespace folder
        └── [recipe-name].yaml # Your custom recipe manifest
```

*Example:* `recipes/arnav/qwen3-30b-moe-8gb-cpu-offload.yaml`

---

## How to Submit a Recipe (GitOps Flow)

1. **Fork the Repository:** Create a personal fork of the `bloc-product` repository.
2. **Create Your Directory:** If you haven't published before, create a folder under `/recipes` named exactly after your GitHub username (e.g., `/recipes/my-github-username/`).
3. **Write Your Recipe:** Copy `recipes/TEMPLATE.yaml` into your directory, rename it (e.g., `qwen-7b-gaming.yaml`), and fill out the configuration.
4. **Test Locally:** Use the Monaco editor on the submit page at `bloc-hub.com/registry/submit` to check for YAML syntax validity and layout mapping.
5. **Open a Pull Request:** Submit a PR to merge your recipe file into the `main` branch.
6. **CI Validation:** Our automated pipeline will verify:
   * That you only modified files inside `recipes/[your-github-username]/`.
   * That the recipe matches the `bloc/v1` schema rules.
   * That the Hugging Face GGUF model download links are fully reachable.
7. **Merge & Live Sync:** Once a maintainer reviews and merges the PR, our GitHub Action automatically parses the YAML file and updates the registry database. Your recipe will immediately appear live at `bloc-hub.com/recipes/[your-username]/[recipe-name]`.

---

## Recipe Structure Overview

Every recipe is composed of two distinct parts:

### Layer 1: Registry Metadata
This section is read by the **Bloc Hub Website** to power the search engine, filtering UI, and the recipe's detail page.
* **Fields:** `schema`, `metadata`, `model`, `engine`, `hardware`
* **Constraint:** All fields here must be accurate to ensure proper filtering (e.g., min_vram, target_platform).

### Layer 2: Engine Configuration
This section is read by the **Bloc CLI** when executing `bloc deploy`. The website stores this verbatim without interpreting it.
* **Fields:** `engine_config`, `pre_run`
* **Constraint:** Be explicit about every flag. Do not rely on default parameters as they may change across different versions of `llama.cpp`.

For detailed information on configuring each flag, consult our documentation or the annotations in [TEMPLATE.yaml](file:///Users/arnavgautam/Documents/bloc/bloc-product/recipes/TEMPLATE.yaml).
