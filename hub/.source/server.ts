// @ts-nocheck
import { default as __fd_glob_2 } from "../content/docs/meta.json?collection=meta"
import * as __fd_glob_1 from "../content/docs/index.mdx?collection=docs"
import * as __fd_glob_0 from "../content/blog/announcing-bloc.mdx?collection=blog"
import { server } from 'fumadocs-mdx/runtime/server';
import type * as Config from '../source.config';

const create = server<typeof Config, import("fumadocs-mdx/runtime/types").InternalTypeConfig & {
  DocData: {
  }
}>({"doc":{"passthroughs":["extractedReferences"]}});

export const blog = await create.doc("blog", "content/blog", {"announcing-bloc.mdx": __fd_glob_0, });

export const docs = await create.doc("docs", "content/docs", {"index.mdx": __fd_glob_1, });

export const meta = await create.meta("meta", "content/docs", {"meta.json": __fd_glob_2, });