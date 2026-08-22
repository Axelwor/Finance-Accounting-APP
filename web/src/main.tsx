import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// M3 dynamic color: apply the user's persisted source-color theme before
// first paint (static m3-tokens.css is the fallback for the default color).
import { initM3Theme } from "./lib/m3-theme";
// M3 stylesheet system: tokens + shell, shared components, screen patterns.
import "./styles/base.css";
import "./styles/components.css";
import "./styles/screens.css";
import "./styles/legacy-forms.css";
import "./styles/print.css";
import App from "./App";

initM3Theme();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
