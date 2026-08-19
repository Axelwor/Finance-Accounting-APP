import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// M3 dynamic color: apply the user's persisted source-color theme before
// first paint (static m3-tokens.css is the fallback for the default color).
import { initM3Theme } from "./lib/m3-theme";
// Modular styles (m-018): split from the monolithic styles.css, imported in
// the original order to preserve the cascade.
import "./styles/base.css";
import "./styles/shell.css";
import "./styles/components.css";
import "./styles/auth.css";
import "./styles/workbench.css";
import "./styles/features.css";
import App from "./App";

initM3Theme();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
