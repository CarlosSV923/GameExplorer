/* eslint-disable react-refresh/only-export-components -- createLink wraps components */
import { createLink } from '@tanstack/react-router'
import { forwardRef, type AnchorHTMLAttributes } from 'react'

import { backLinkClass, cx, LinkButton } from '@/shared/ui'

/** LinkButton as a typed router link (client-side navigation). */
export const ButtonLink = createLink(LinkButton)

const BackAnchor = forwardRef<HTMLAnchorElement, AnchorHTMLAttributes<HTMLAnchorElement>>(
  function BackAnchor({ className, ...rest }, ref) {
    return <a ref={ref} className={cx(backLinkClass, className)} {...rest} />
  },
)

/** The 44 px "Back" link of a page header, as a router link. */
export const BackLink = createLink(BackAnchor)
