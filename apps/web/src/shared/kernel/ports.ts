import { createContext, use, type Context } from 'react'

/**
 * A context that carries one module's ports (its adapters). The composition
 * root (src/app) provides them; the module's hooks and screens read them and
 * never know whether they talk to the API or to the demo.
 */
export function createPortContext<T>(name: string): [Context<T | null>, () => T] {
  const context = createContext<T | null>(null)
  context.displayName = name
  const usePorts = () => {
    const ports = use(context)
    if (!ports) throw new Error(`${name} ports are not provided`)
    return ports
  }
  return [context, usePorts]
}
