/**
 * Node loader that appends `.js` to extensionless relative imports.
 * Needed because @material/material-color-utilities@0.4.0 ships extensionless
 * internal ESM imports (fine for bundlers like Vite, not for plain Node).
 *
 * Usage: node --import ./scripts/register-m3-loader.mjs <script> [args]
 */
import { register } from "node:module";
register(new URL("./m3-extensionless-loader.mjs", import.meta.url));
