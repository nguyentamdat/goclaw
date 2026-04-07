// Strip only '-dirty' suffix from version string, keep git describe info.
// "v2.67.1-28-g0aa8e1a3-dirty" → "v2.67.1-28-g0aa8e1a3"
// "v2.67.1" → "v2.67.1", "dev" → "dev"
  export function cleanVersion(v: string): string {
  return v.replace(/-dirty$/, "");
}
