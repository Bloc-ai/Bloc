// @ts-nocheck
import { browser } from 'fumadocs-mdx/runtime/browser';
import type * as Config from '../source.config';

const create = browser<typeof Config, import("fumadocs-mdx/runtime/types").InternalTypeConfig & {
  DocData: {
  }
}>();
const browserCollections = {
  blog: create.doc("blog", {"announcing-bloc.mdx": () => import("../content/blog/announcing-bloc.mdx?collection=blog"), }),
  docs: create.doc("docs", {"index.mdx": () => import("../content/docs/index.mdx?collection=docs"), }),
};
export default browserCollections;