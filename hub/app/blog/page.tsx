import Link from "next/link";
import { blogSource } from "@/lib/source";

function BlogHighlight({ children }: { children: React.ReactNode }) {
  return (
    <span 
      className="inline text-black dark:text-white px-2 box-decoration-clone"
      style={{
        fontFamily: 'inherit',
        background: 'linear-gradient(to bottom, transparent 15%, #2563EB 15%, #2563EB 82%, transparent 82%)'
      }}
    >
      {children}
    </span>
  );
}

export default function BlogIndex() {
  const posts = [...blogSource.getPages()].sort(
    (a, b) => new Date(b.data.date).getTime() - new Date(a.data.date).getTime()
  );

  return (
    <div className="max-w-6xl w-full mx-auto px-6 py-12 select-none">
      
      {/* Header Section */}
      <div className="w-full text-left mt-12 mb-16">
        <h1 className="text-4xl md:text-5xl font-medium tracking-tight font-switzer text-black dark:text-white mb-0">
          <BlogHighlight>Bloc Hub Blog</BlogHighlight>
        </h1>
      </div>

      {/* Blog Feed Grid */}
      {posts.length === 0 ? (
        <div className="w-full border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3]/50 dark:bg-[#171616]/50 py-16 text-center rounded-none">
          <p className="font-mono text-xs text-zinc-400 dark:text-zinc-500">No blog posts found. Check back soon!</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 w-full">
          {posts.map((post) => {
            const formattedDate = new Date(post.data.date).toLocaleDateString("en-US", {
              year: "numeric",
              month: "short",
              day: "numeric",
            });

            return (
              <Link 
                key={post.url} 
                href={post.url}
                className="group relative flex flex-col justify-between border border-zinc-300 dark:border-zinc-800 bg-[#f6f6f3] dark:bg-[#171616] p-6 rounded-none transition-colors duration-200 hover:bg-[#ededeb] dark:hover:bg-[#201f1f] cursor-pointer min-h-[170px]"
              >
                {/* SVG Corner L-Brackets on Hover */}
                {/* Top-Left */}
                <svg 
                  viewBox="0 0 12 12" 
                  className="absolute top-0 left-0 w-3 h-3 fill-black dark:fill-white pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:-translate-y-1.5"
                >
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                {/* Top-Right */}
                <svg 
                  viewBox="0 0 12 12" 
                  className="absolute top-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:-translate-y-1.5"
                >
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                {/* Bottom-Left */}
                <svg 
                  viewBox="0 0 12 12" 
                  className="absolute bottom-0 left-0 w-3 h-3 fill-black dark:fill-white scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 -translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:-translate-x-1.5 group-hover:translate-y-1.5"
                >
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>
                {/* Bottom-Right */}
                <svg 
                  viewBox="0 0 12 12" 
                  className="absolute bottom-0 right-0 w-3 h-3 fill-black dark:fill-white scale-x-[-1] scale-y-[-1] pointer-events-none transition-all duration-200 ease-out opacity-0 translate-x-0.5 translate-y-0.5 group-hover:opacity-100 group-hover:translate-x-1.5 group-hover:translate-y-1.5"
                >
                  <path d="M 0,12 L 0,0 L 12,0 L 12,1 L 4,1 Q 1,1 1,4 L 1,12 Z" />
                </svg>

                {/* Content */}
                <div>
                  <div className="flex items-center gap-2 mb-3">
                    <span className="font-mono text-[9px] uppercase tracking-wider px-2 py-0.5 border border-zinc-300 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400">
                      {post.data.tag || "Systems"}
                    </span>
                    <span className="font-mono text-[9px] text-zinc-400 dark:text-zinc-600">
                      {formattedDate}
                    </span>
                  </div>
                  
                  <h2 className="text-xl font-semibold tracking-tight font-switzer text-black dark:text-white mb-0 leading-snug">
                    {post.data.title}
                  </h2>
                </div>

                <div className="flex items-center justify-between mt-6 border-t border-zinc-200 dark:border-zinc-800/80 pt-4 font-mono text-[10px] text-zinc-400 dark:text-zinc-500 group-hover:text-black dark:group-hover:text-white transition-colors">
                  <span>By {post.data.authors.join(", ")}</span>
                  <span className="flex items-center gap-1">
                    Read Post 
                    <svg width="8" height="8" viewBox="0 0 10 10" fill="none" xmlns="http://www.w3.org/2000/svg" className="transform group-hover:translate-x-0.5 transition-transform">
                      <path d="M1 9L9 1M9 1H1M9 1V9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                    </svg>
                  </span>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
