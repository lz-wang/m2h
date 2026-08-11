// KaTeX ships its own `katex` types but the `katex/contrib/auto-render`
// subpath has no bundled declaration. Declare the subset of the auto-render
// API m2h relies on so strict TypeScript keeps working.
declare module "katex/contrib/auto-render" {
  export interface AutoRenderDelimiter {
    left: string;
    right: string;
    display?: boolean;
  }

  export interface AutoRenderOptions {
    delimiters?: AutoRenderDelimiter[];
    preProcess?: (math: string) => string;
    ignoredTags?: string[];
    ignoredClasses?: string[];
    errorCallback?: (message: string, error: Error) => void;
    displayMode?: boolean;
    macros?: Record<string, string>;
    // KaTeX render options that auto-render forwards to katex.render.
    throwOnError?: boolean;
    strict?: boolean | string;
    errorColor?: string;
  }

  function renderMathInElement(
    element: HTMLElement,
    options?: AutoRenderOptions,
  ): void;

  export default renderMathInElement;
  export { renderMathInElement };
}
