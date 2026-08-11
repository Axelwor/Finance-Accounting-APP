import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// Modular styles (m-018): split from the monolithic styles.css, imported in
// the original order to preserve the cascade.
import "./styles/base.css";
import "./styles/shell.css";
import "./styles/components.css";
import "./styles/auth.css";
import "./styles/workbench.css";
import "./styles/features.css";
import App from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
