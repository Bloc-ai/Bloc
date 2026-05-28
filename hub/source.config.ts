import { defineDocs, defineCollections, defineConfig, frontmatterSchema } from 'fumadocs-mdx/config';
import { z } from 'zod';

// 1. Define standard documentation (will parse content/docs/)
export const { docs, meta } = defineDocs({
  dir: 'content/docs',
});

// 2. Define blog posts collection (will parse content/blog/)
export const blog = defineCollections({
  type: 'doc',
  dir: 'content/blog',
  schema: frontmatterSchema.extend({
    authors: z.array(z.string()).default(["Bloc Team"]),
    date: z.string().or(z.date()).transform((val) => new Date(val)),
    tag: z.string().optional(),
    draft: z.boolean().default(false),
  }),
});

// 3. Global configuration
export default defineConfig({
  mdxOptions: {
    remarkPlugins: [],
    rehypePlugins: [],
  },
});
