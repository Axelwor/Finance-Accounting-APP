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

// jsdom does not implement ElementInternals' form-associated API, which
// @material/web text fields (and other form components) call from their
// constructors via attachInternals(). Stub the missing methods so md-*
// elements can be instantiated in jsdom.
if (typeof HTMLElement !== "undefined" && HTMLElement.prototype.attachInternals) {
  const originalAttachInternals = HTMLElement.prototype.attachInternals;
  HTMLElement.prototype.attachInternals = function () {
    const internals = originalAttachInternals.call(this);
    if (typeof internals.setFormValue !== "function") {
      internals.setFormValue = () => {};
    }
    if (typeof internals.setValidity !== "function") {
      internals.setValidity = () => {};
    }
    if (typeof internals.reportValidity !== "function") {
      internals.reportValidity = () => true;
    }
    if (typeof internals.checkValidity !== "function") {
      internals.checkValidity = () => true;
    }
    return internals;
  };
}
