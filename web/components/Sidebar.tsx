"use client";

import clsx from "clsx";
import {
  Boxes,
  CalendarClock,
  Cpu,
  LayoutDashboard,
  Server,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
}

const navItems: NavItem[] = [
  { href: "/", label: "Overview", icon: LayoutDashboard },
  { href: "/jobs", label: "Jobs", icon: Boxes },
  { href: "/nodes", label: "Nodes", icon: Server },
  { href: "/events", label: "Events", icon: CalendarClock },
];

function isActive(pathname: string, href: string): boolean {
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="sticky top-0 hidden h-screen w-60 shrink-0 flex-col border-r border-slate-200 bg-white/80 backdrop-blur dark:border-slate-800 dark:bg-slate-950/80 lg:flex">
      <div className="flex items-center gap-2.5 px-5 py-5">
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg bg-brand-600 text-white shadow-sm">
          <Cpu className="h-5 w-5" aria-hidden />
        </span>
        <div className="leading-tight">
          <p className="text-sm font-semibold text-slate-900 dark:text-slate-50">
            GPU Scheduler
          </p>
          <p className="text-[11px] text-slate-500 dark:text-slate-400">
            Cluster Control Plane
          </p>
        </div>
      </div>

      <nav className="flex-1 space-y-1 px-3 py-2">
        {navItems.map((item) => {
          const active = isActive(pathname, item.href);
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={active ? "page" : undefined}
              className={clsx(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                active
                  ? "bg-brand-50 text-brand-700 dark:bg-brand-500/15 dark:text-brand-300"
                  : "text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800/60 dark:hover:text-slate-100",
              )}
            >
              <Icon className="h-4.5 w-4.5 shrink-0" aria-hidden />
              {item.label}
            </Link>
          );
        })}
      </nav>

      <div className="px-5 py-4 text-[11px] text-slate-400 dark:text-slate-600">
        v0.1.0 · production
      </div>
    </aside>
  );
}

// Compact horizontal nav used on small screens.
export function MobileNav() {
  const pathname = usePathname();

  return (
    <nav className="flex items-center gap-1 overflow-x-auto border-b border-slate-200 bg-white px-3 py-2 dark:border-slate-800 dark:bg-slate-950 lg:hidden">
      {navItems.map((item) => {
        const active = isActive(pathname, item.href);
        const Icon = item.icon;
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? "page" : undefined}
            className={clsx(
              "flex items-center gap-2 whitespace-nowrap rounded-lg px-3 py-1.5 text-sm font-medium transition-colors",
              active
                ? "bg-brand-50 text-brand-700 dark:bg-brand-500/15 dark:text-brand-300"
                : "text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800/60",
            )}
          >
            <Icon className="h-4 w-4" aria-hidden />
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
