// @ts-nocheck
import { default as __fd_glob_14 } from "../content/docs/meta.json?collection=meta"
import * as __fd_glob_13 from "../content/docs/engines/vllm.mdx?collection=docs"
import * as __fd_glob_12 from "../content/docs/engines/sglang.mdx?collection=docs"
import * as __fd_glob_11 from "../content/docs/engines/llama-cpp.mdx?collection=docs"
import * as __fd_glob_10 from "../content/docs/troubleshooting.mdx?collection=docs"
import * as __fd_glob_9 from "../content/docs/recipes.mdx?collection=docs"
import * as __fd_glob_8 from "../content/docs/quickstart.mdx?collection=docs"
import * as __fd_glob_7 from "../content/docs/publishing.mdx?collection=docs"
import * as __fd_glob_6 from "../content/docs/local-recipes.mdx?collection=docs"
import * as __fd_glob_5 from "../content/docs/installation.mdx?collection=docs"
import * as __fd_glob_4 from "../content/docs/index.mdx?collection=docs"
import * as __fd_glob_3 from "../content/docs/how-bloc-works.mdx?collection=docs"
import * as __fd_glob_2 from "../content/docs/engines-and-runtimes.mdx?collection=docs"
import * as __fd_glob_1 from "../content/docs/commands.mdx?collection=docs"
import * as __fd_glob_0 from "../content/blog/rethinking-local-ai-engine.mdx?collection=blog"
import { server } from 'fumadocs-mdx/runtime/server';
import type * as Config from '../source.config';

const create = server<typeof Config, import("fumadocs-mdx/runtime/types").InternalTypeConfig & {
  DocData: {
  }
}>({"doc":{"passthroughs":["extractedReferences"]}});

export const blog = await create.doc("blog", "content/blog", {"rethinking-local-ai-engine.mdx": __fd_glob_0, });

export const docs = await create.doc("docs", "content/docs", {"commands.mdx": __fd_glob_1, "engines-and-runtimes.mdx": __fd_glob_2, "how-bloc-works.mdx": __fd_glob_3, "index.mdx": __fd_glob_4, "installation.mdx": __fd_glob_5, "local-recipes.mdx": __fd_glob_6, "publishing.mdx": __fd_glob_7, "quickstart.mdx": __fd_glob_8, "recipes.mdx": __fd_glob_9, "troubleshooting.mdx": __fd_glob_10, "engines/llama-cpp.mdx": __fd_glob_11, "engines/sglang.mdx": __fd_glob_12, "engines/vllm.mdx": __fd_glob_13, });

export const meta = await create.meta("meta", "content/docs", {"meta.json": __fd_glob_14, });