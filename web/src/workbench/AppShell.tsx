import { Outlet } from "react-router-dom";
import { TopBar } from "./TopBar";
import { Sidebar } from "./Sidebar";
import { TabStrip } from "./TabStrip";
import { WorkArea } from "./WorkArea";

/**
 * AppShell — fixed top bar, fixed left sidebar with hover popups, a
 * horizontal tab strip, and the active tab content fills the rest.
 */
export function AppShell() {
  return (
    <div className="app-shell">
      <TopBar />
      <Sidebar />
      <main className="app-main" role="main">
        <TabStrip />
        <div className="app-main__inner">
          <Outlet />
        </div>
        <WorkArea />
      </main>
    </div>
  );
}
