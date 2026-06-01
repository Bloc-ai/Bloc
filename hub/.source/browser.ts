// @ts-nocheck
import { browser } from 'fumadocs-mdx/runtime/browser';
import type * as Config from '../source.config';

const create = browser<typeof Config, import("fumadocs-mdx/runtime/types").InternalTypeConfig & {
  DocData: {
  }
}>();
const browserCollections = {
  blog: create.doc("blog", {"rethinking-local-ai-engine.mdx": () => import("../content/blog/rethinking-local-ai-engine.mdx?collection=blog"), }),
  docs: create.doc("docs", {"commands.mdx": () => import("../content/docs/commands.mdx?collection=docs"), "engines-and-runtimes.mdx": () => import("../content/docs/engines-and-runtimes.mdx?collection=docs"), "how-bloc-works.mdx": () => import("../content/docs/how-bloc-works.mdx?collection=docs"), "index.mdx": () => import("../content/docs/index.mdx?collection=docs"), "installation.mdx": () => import("../content/docs/installation.mdx?collection=docs"), "local-recipes.mdx": () => import("../content/docs/local-recipes.mdx?collection=docs"), "publishing.mdx": () => import("../content/docs/publishing.mdx?collection=docs"), "quickstart.mdx": () => import("../content/docs/quickstart.mdx?collection=docs"), "recipes.mdx": () => import("../content/docs/recipes.mdx?collection=docs"), "troubleshooting.mdx": () => import("../content/docs/troubleshooting.mdx?collection=docs"), }),
};
export default browserCollections;