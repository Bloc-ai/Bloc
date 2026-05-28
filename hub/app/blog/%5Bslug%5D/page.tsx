import Link from "next/link";
import { blogSource } from "@/lib/source";
import { notFound } from "next/navigation";
import type { Metadata } from "next";

export default async function BlogPostPage(props: {
  params: Promise<{ slug: string }>;
}) {
  const params = await props.params;
  const page = blogSource.getPage([params.slug]);
  if (!page) notFound();

  const MDX = page.data.body;
  const formattedDate = new Date(page.data.date).toLocaleDateString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });

  return (
    <article className="max-w-3xl w-full mx-auto px-6 py-16">
      {/* Back to Blog */}
      <Link href="/blog" className="inline-flex items-center gap-1.5 font-mono text-[10px] text-zinc-400 dark:text-zinc-500 hover:text-black dark:hover:text-white transition-colors mb-12 select-none">
        <svg width="8" height="8" viewBox="0 0 10 10" fill="none" xmlns="http://www.w3.org/2000/svg" className="transform rotate-180">
          <path d="M1 9L9 1M9 1H1M9 1V9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
        Back to blog
      </Link>

      {/* Post Header */}
      <header className="mb-12 border-b border-zinc-200 dark:border-zinc-800/80 pb-8 select-none">
        <div className="flex items-center gap-2 mb-4">
          <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400">
            {page.data.tag || "Systems"}
          </span>
          <span className="font-mono text-[9px] text-zinc-400 dark:text-zinc-600">
            {formattedDate}
          </span>
        </div>
        
        <h1 className="text-4xl md:text-5xl font-semibold tracking-tight font-switzer text-black dark:text-white mb-6 leading-tight">
          {page.data.title}
        </h1>
        
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-full border border-zinc-300 dark:border-zinc-800 bg-zinc-200 dark:bg-zinc-900 flex items-center justify-center font-mono text-[10px] font-bold text-zinc-600 dark:text-zinc-400">
            {page.data.authors[0][0]}
          </div>
          <div className="flex flex-col font-mono text-[10px]">
            <span className="text-zinc-800 dark:text-zinc-200 font-bold">{page.data.authors.join(", ")}</span>
            <span className="text-zinc-400 dark:text-zinc-500">Bloc Core Engineering</span>
          </div>
        </div>
      </header>

      {/* Post Body (MDX Content) */}
      <div className="prose prose-zinc dark:prose-invert max-w-none font-sans text-sm md:text-base leading-relaxed text-zinc-800 dark:text-zinc-200 prose-headings:font-switzer prose-headings:font-semibold prose-headings:text-black dark:prose-headings:text-white prose-a:text-blue-600 prose-code:font-mono prose-code:text-xs">
        <MDX />
      </div>
    </article>
  );
}

export async function generateStaticParams() {
  return blogSource.generateParams().map((p) => ({ slug: p.slug[0] }));
}

export async function generateMetadata(props: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const params = await props.params;
  const page = blogSource.getPage([params.slug]);
  if (!page) notFound();

  return {
    title: `${page.data.title} - Bloc Blog`,
    description: page.data.description,
  };
}
