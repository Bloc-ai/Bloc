"use client";

import { DocsLayout, useDocsLayout } from "fumadocs-ui/layouts/docs";
import type { ReactNode } from "react";
import { source } from "@/lib/source";
import { Menu, X } from "lucide-react";

function MobileSidebarToggle() {
  const { slots } = useDocsLayout();
  const { open, setOpen } = slots.sidebar.useSidebar();

  return (
    <button
      onClick={() => setOpen(!open)}
      className="md:hidden fixed bottom-6 right-6 z-50 flex items-center justify-center w-12 h-12 bg-blue-600 text-white rounded-full shadow-lg border border-blue-500/20 active:scale-95 transition-all duration-200 cursor-pointer"
      aria-label="Toggle Documentation Sidebar"
    >
      {open ? (
        <X className="w-5 h-5" />
      ) : (
        <Menu className="w-5 h-5" />
      )}
    </button>
  );
}

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <DocsLayout 
      tree={source.pageTree} 
      nav={{ enabled: false }}
      searchToggle={{ enabled: false }}
      sidebar={{ collapsible: false }}
      themeSwitch={{ enabled: false }}
    >
      {children}
      <MobileSidebarToggle />
    </DocsLayout>
  );
}
