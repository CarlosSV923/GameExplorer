/** Lower case without accents: "Pokémon" matches "pokemon" (RF-22). */
export function fold(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase()
}

/** Every word of the query appears in the text, ignoring case and accents. */
export function matchesWords(text: string, query: string): boolean {
  const words = fold(query).split(/\s+/).filter(Boolean)
  const haystack = fold(text)
  return words.every((w) => haystack.includes(w))
}

/** The last segment of a path ("a/b/c.nsp" → "c.nsp"). */
export function baseName(path: string): string {
  return path.slice(path.lastIndexOf('/') + 1)
}
