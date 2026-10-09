const urls = new Map<string, string>()

/**
 * The demo has no files: a download is a short note (RF-65). Each link is a
 * Blob URL (one per name), so it also works when the browser navigates to it.
 */
export function downloadNote(name: string): string {
  const known = urls.get(name)
  if (known) return known
  const text = [
    'GameExplorer · demo',
    '',
    `ES: Esto es una demo sin servidor: «${name}» no existe. En una instalación real`,
    'se descargaría el archivo (o el juego como .zip), reanudable si se corta.',
    '',
    `EN: This is a demo without a server: "${name}" does not exist. A real install`,
    'would download the file (or the game as a .zip), resumable if it is interrupted.',
    '',
  ].join('\n')
  const url = URL.createObjectURL(new Blob([text], { type: 'text/plain;charset=utf-8' }))
  urls.set(name, url)
  return url
}
