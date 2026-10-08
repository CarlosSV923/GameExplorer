import type { SVGProps } from 'react'

/** Stroke icons from the mockups; decorative (aria-hidden) by default. */
function icon(paths: string[], fill = false) {
  return function Icon({ size = 18, ...rest }: SVGProps<SVGSVGElement> & { size?: number }) {
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill={fill ? 'currentColor' : 'none'}
        stroke={fill ? 'none' : 'currentColor'}
        strokeWidth={2.2}
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
        focusable="false"
        {...rest}
      >
        {paths.map((d) => (
          <path key={d} d={d} />
        ))}
      </svg>
    )
  }
}

export const SearchIcon = icon(['M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14Z', 'm20 20-3.5-3.5'])
export const UploadIcon = icon([
  'M12 16V4',
  'M7 9l5-5 5 5',
  'M4 16v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3',
])
export const DownloadIcon = icon(['M12 3v12', 'M7 10l5 5 5-5', 'M5 21h14'])
export const TrashIcon = icon(['M4 7h16', 'M9 7V4h6v3', 'M6 7l1 13h10l1-13'])
export const RestoreIcon = icon(['M3 12a9 9 0 1 0 3-6.7L3 8', 'M3 3v5h5'])
export const RematchIcon = icon(['M21 12a9 9 0 1 1-3-6.7L21 8', 'M21 3v5h-5'])
export const ChevronLeftIcon = icon(['m15 18-6-6 6-6'])
export const ChevronRightIcon = icon(['m9 18 6-6-6-6'])
export const ChevronUpIcon = icon(['m6 15 6-6 6 6'])
export const ChevronDownIcon = icon(['m6 9 6 6 6-6'])
export const CloseIcon = icon(['M6 6l12 12', 'M18 6 6 18'])
export const CheckIcon = icon(['m5 12 5 5L20 7'])
export const MenuIcon = icon(['M4 6h16', 'M4 12h16', 'M4 18h16'])
export const EditIcon = icon(['M4 20h4L19 9l-4-4L4 16z'])
export const LockIcon = icon([
  'M6 11h12a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-6a2 2 0 0 1 2-2Z',
  'M8 11V7a4 4 0 0 1 8 0v4',
])
export const WarningIcon = icon(['M12 3 2 21h20z', 'M12 10v5', 'M12 18v.5'])
export const ErrorIcon = icon(['M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z', 'M12 7v6', 'M12 16.5v.5'])
export const ClockIcon = icon(['M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z', 'M12 7v5l3 2'])
export const FolderIcon = icon([
  'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z',
])
