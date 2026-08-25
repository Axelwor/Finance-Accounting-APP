import { useEffect, useState } from "react";
import { Outlet } from "react-router-dom";
import { TopBar } from "./TopBar";
import { Sidebar } from "./Sidebar";
import { TabStrip } from "./TabStrip";
import { WorkArea } from "./WorkArea";
import { CommandPalette } from "../components/CommandPalette";

/**
 * AppShell — fixed top bar, fixed left sidebar with hover popups, a
 * horizontal tab strip, and the active tab content fills the rest.
 */
export function AppShell() {
  const [paletteOpen, setPaletteOpen] = useState(false);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((prev) => !prev);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  return (
    <div className="app-shell">
      <Sidebar />
      <div className="app-main" role="main" id="app-main">
        <TopBar onOpenPalette={() => setPaletteOpen(true)} />
        <TabStrip />
        <div className="app-main__inner">
          <Outlet />
          <WorkArea />
        </div>
      </div>
      <CommandPalette isOpen={paletteOpen} onClose={() => setPaletteOpen(false)} />
    </div>
  );
}

