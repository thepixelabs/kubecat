// Ambient declaration for CSS side-effect imports.
//
// TypeScript 6.0 stopped auto-typing side-effect imports of asset files
// (TS2882). Vite still resolves these correctly at build/dev time; this
// declaration only exists to make the TypeScript checker happy.
//
// No-op on TypeScript 5.x.

declare module "*.css";
