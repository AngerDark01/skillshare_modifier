/**
 * Convert a glob pattern (supporting * and ?) into a RegExp.
 * If the pattern contains no glob characters, it becomes a case-insensitive substring match.
 */
export function globToRegex(pattern: string): RegExp {
  if (!/[*?]/.test(pattern)) {
    const escaped = pattern.replace(/[.+^${}()|[\]\\]/g, '\\$&');
    return new RegExp(escaped, 'i');
  }
  const escaped = pattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '.*')
    .replace(/\?/g, '.');
  return new RegExp(`^${escaped}$`, 'i');
}
