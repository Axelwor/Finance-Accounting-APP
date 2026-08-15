import "@testing-library/jest-dom/vitest";

// jsdom lacks matchMedia — stub it for components that probe media queries.
if (!window.matchMedia) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

// jsdom implements scrollIntoView as a no-op absence — stub it for the
// Combobox (and any other component that scrolls the active option into
// view).
if (typeof Element !== "undefined" && !Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {};
}
