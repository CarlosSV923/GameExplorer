import { dataSource } from './config'

export function App() {
  return (
    <main>
      <h1>GameExplorer</h1>
      <p>Modo: {dataSource}</p>
    </main>
  )
}
