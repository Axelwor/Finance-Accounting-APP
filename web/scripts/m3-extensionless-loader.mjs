/** Retry failed relative imports with a `.js` extension appended. */
export async function resolve(specifier, context, nextResolve) {
  try {
    return await nextResolve(specifier, context);
  } catch (err) {
    if (
      err?.code === "ERR_MODULE_NOT_FOUND" &&
      (specifier.startsWith(".") || specifier.startsWith("/"))
    ) {
      return nextResolve(`${specifier}.js`, context);
    }
    throw err;
  }
}
