import { useEffect, useState } from 'react'
import type { SimulationState } from './types'

// EventSource は接続切れを自動で再接続するため、再接続ロジックは不要。
export function useSimulationStream(): SimulationState | null {
  const [state, setState] = useState<SimulationState | null>(null)

  useEffect(() => {
    // api.ts と同じ規約: VITE_API_BASE が設定されていれば別オリジンに向ける
    // （trim も api.ts に合わせる。env に空白が紛れたとき挙動が割れないように）。
    const base = (import.meta.env.VITE_API_BASE ?? '').trim().replace(/\/$/, '')
    const es = new EventSource(`${base}/events`)
    es.addEventListener('tick', (ev) => {
      try {
        setState(JSON.parse(ev.data) as SimulationState)
      } catch (err) {
        console.error('parse tick event', err)
      }
    })
    es.onerror = (err) => console.error('event source error', err)
    return () => es.close()
  }, [])

  return state
}
