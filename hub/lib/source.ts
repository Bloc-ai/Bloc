import { toFumadocsSource } from "fumadocs-mdx/runtime/server";
import { loader } from "fumadocs-core/source";
import { docs, meta, blog as blogCollection } from "@/.source/server";

// Main Docs Loader
export const source = loader({
  baseUrl: "/docs",
  source: toFumadocsSource(docs, meta),
});

// Main Blog Loader
export const blogSource = loader({
  baseUrl: "/blog",
  source: toFumadocsSource(blogCollection, []),
});
