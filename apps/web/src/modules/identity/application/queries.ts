import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { useIdentityPorts, type IdentityPorts } from './ports'

export const sessionKey = ['session'] as const

/** The session, checked before every protected route. */
export function sessionQuery(ports: IdentityPorts) {
  return queryOptions({
    queryKey: sessionKey,
    queryFn: () => ports.session(),
    staleTime: 60_000,
    retry: false,
  })
}

export function useLogin() {
  const ports = useIdentityPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (password: string) => ports.login(password),
    onSuccess: () => client.invalidateQueries({ queryKey: sessionKey }),
  })
}

export function useLogout() {
  const ports = useIdentityPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: () => ports.logout(),
    onSuccess: () => {
      client.clear()
    },
  })
}
