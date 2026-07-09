import type { Metadata } from "next";
import type { ReactNode } from "react";
import { MobileNav, Sidebar } from "@/components/Sidebar";
import { TopBar } from "@/components/TopBar";
import "./globals.css";

export const metadata: Metadata = {
  title: "GPU Cluster Scheduler",
  description:
    "Production dashboard for monitoring and controlling a GPU cluster scheduler.",
};

// Prevent a theme flash by setting the class before hydration.
const themeScript = `(function(){try{var t=localStorage.getItem('gcs-theme');if(!t){t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}if(t==='dark'){document.documentElement.classList.add('dark');}}catch(e){}})();`;

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>
        <div className="flex min-h-screen">
          <Sidebar />
          <div className="flex min-w-0 flex-1 flex-col">
            <TopBar />
            <MobileNav />
            <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 md:px-6 lg:px-8">
              {children}
            </main>
          </div>
        </div>
      </body>
    </html>
  );
}
