import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

function App() {
  return (
    <main className="shell">
      <p className="eyebrow">PEMBUKUAN MUDAH</p>
      <h1>Fondasi aplikasi siap dibangun.</h1>
      <p className="lede">
        Base repository sudah aktif. Fitur MVP akan ditambahkan melalui task
        ledger yang terisolasi.
      </p>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
