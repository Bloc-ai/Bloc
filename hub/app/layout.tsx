import type { Metadata } from "next";
import { Geist_Mono, Silkscreen, Geist } from "next/font/google";
import "./globals.css";
import Navbar from "@/components/Navbar";
import { RootProvider } from "fumadocs-ui/provider/next";
import ThemeShortcut from "@/components/ThemeShortcut";
import { AuthProvider } from "@/context/AuthContext";
import { Toaster } from "sonner";
import { Analytics } from "@vercel/analytics/next";

const geist = Geist({subsets:['latin'],variable:'--font-sans'});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

const silkscreen = Silkscreen({
  variable: "--font-silkscreen",
  weight: ["400"],
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Bloc",
  description: "The Docker Hub for local AI models. Pull and run optimized LLMs instantly.",
  metadataBase: new URL("https://bloc-theta.vercel.app"),
  icons: {
    icon: "/images/favicon.ico",
  },
  openGraph: {
    title: "Bloc Hub",
    description: "The local-first model registry and runner toolchain.",
    url: "https://bloc-theta.vercel.app",
    siteName: "Bloc Hub",
    images: [
      {
        url: "/images/Bloc-card-large.png",
        width: 1200,
        height: 630,
        alt: "Bloc Hub Link Preview Cover",
      },
    ],
    locale: "en_US",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Bloc Hub",
    description: "Pull and run optimized local AI environments instantly.",
    images: ["/images/Bloc-card-large.png"],
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${geistMono.variable} ${silkscreen.variable} ${geist.variable} h-full antialiased font-sans`}
      suppressHydrationWarning
    >
      <head>
        <link href="https://api.fontshare.com/v2/css?f[]=switzer@100,200,300,400,500,600,700,800,900&display=swap" rel="stylesheet" />
      </head>
      <body className="min-h-full flex flex-col bg-background text-foreground">
        <RootProvider theme={{ defaultTheme: "system", enableSystem: true }}>
          <AuthProvider>
            <ThemeShortcut />
            <Navbar />
            <main className="flex-grow flex flex-col pt-12">
              {children}
            </main>
            <Toaster position="bottom-right" theme="system" />
            <Analytics />
          </AuthProvider>
        </RootProvider>
      </body>
    </html>
  );
}
