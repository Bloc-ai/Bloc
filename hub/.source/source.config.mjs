// source.config.ts
import { defineDocs, defineCollections, defineConfig, frontmatterSchema } from "fumadocs-mdx/config";
import { z } from "zod";
var { docs, meta } = defineDocs({
  dir: "content/docs"
});
var blog = defineCollections({
  type: "doc",
  dir: "content/blog",
  schema: frontmatterSchema.extend({
    authors: z.array(z.string()).default(["Bloc Team"]),
    date: z.string().or(z.date()).transform((val) => new Date(val)),
    tag: z.string().optional(),
    draft: z.boolean().default(false)
  })
});
var source_config_default = defineConfig({
  mdxOptions: {
    remarkPlugins: [],
    rehypePlugins: []
  }
});
export {
  blog,
  source_config_default as default,
  docs,
  meta
};
