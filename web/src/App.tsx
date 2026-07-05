import { useEffect, useRef, useState } from 'react'
import { useSimulationStream } from './sse'
import { Building } from './components/Building'
import { EventLog, type LogEntry } from './components/EventLog'
import { pressCar, pressHall, resetSimulation } from './api'

const MAX_EVENT_LOG = 100

// optimistic は次の SSE 更新で消えるのが基本だが、即時 served / 即時開扉のように
// サーバ状態に痕跡が残らないケースだと smart 比較では消せない。
// auto-tick 間隔 (既定 1s) を超える保険値を置く。
const OPTIMISTIC_TTL_MS = 2000

export function App() {
  const state = useSimulationStream()

  const [optimisticHall, setOptimisticHall] = useState<Set<string>>(new Set())
  const [optimisticCar, setOptimisticCar] = useState<Map<string, Set<number>>>(new Map())
  // SSE が運ぶ events は「直近の差分」だけ。ログとしての履歴はクライアントで保持する。
  // イベント自体に一意な ID が無いので React key 用の連番をここで振る
  // （timestamp は同一 tick 内で重複し、配列 index は prepend でずれるため）。
  const [eventLog, setEventLog] = useState<LogEntry[]>([])
  const eventSeq = useRef(0)

  useEffect(() => {
    if (!state || !state.events || state.events.length === 0) return
    const stamped = state.events.map((event) => ({ id: eventSeq.current++, event }))
    setEventLog((prev) => [...stamped, ...prev].slice(0, MAX_EVENT_LOG))
  }, [state])

  useEffect(() => {
    if (!state) return
    setOptimisticHall((prev) => {
      if (prev.size === 0) return prev
      const next = new Set<string>()
      for (const key of prev) {
        const [dir, floorStr] = key.split(':')
        const floor = Number(floorStr)
        const direction = dir as 'up' | 'down'
        const confirmed = state.elevators.some((e) =>
          e.assignedHallCalls.some((c) => c.floor === floor && c.direction === direction),
        )
        if (!confirmed) next.add(key)
      }
      return next
    })
    setOptimisticCar((prev) => {
      if (prev.size === 0) return prev
      const next = new Map<string, Set<number>>()
      for (const [eid, floors] of prev) {
        const ev = state.elevators.find((e) => e.id === eid)
        const dests = new Set(ev?.destinationFloors ?? [])
        const remaining = new Set<number>()
        for (const f of floors) {
          if (!dests.has(f)) remaining.add(f)
        }
        if (remaining.size > 0) next.set(eid, remaining)
      }
      return next
    })
  }, [state])

  const removeOptimisticHall = (key: string) =>
    setOptimisticHall((prev) => {
      if (!prev.has(key)) return prev
      const next = new Set(prev)
      next.delete(key)
      return next
    })

  const removeOptimisticCar = (elevatorId: string, floor: number) =>
    setOptimisticCar((prev) => {
      const set = prev.get(elevatorId)
      if (!set || !set.has(floor)) return prev
      const next = new Map(prev)
      const newSet = new Set(set)
      newSet.delete(floor)
      if (newSet.size === 0) next.delete(elevatorId)
      else next.set(elevatorId, newSet)
      return next
    })

  const onHallPress = (floor: number, direction: 'up' | 'down') => {
    const key = hallKey(floor, direction)
    setOptimisticHall((prev) => new Set(prev).add(key))
    const timer = window.setTimeout(() => removeOptimisticHall(key), OPTIMISTIC_TTL_MS)
    pressHall(floor, direction).catch((err) => {
      console.error('pressHall', err)
      window.clearTimeout(timer)
      removeOptimisticHall(key)
    })
  }

  const onCarPress = (elevatorId: string, floor: number) => {
    setOptimisticCar((prev) => {
      const next = new Map(prev)
      const set = new Set(next.get(elevatorId) ?? [])
      set.add(floor)
      next.set(elevatorId, set)
      return next
    })
    const timer = window.setTimeout(
      () => removeOptimisticCar(elevatorId, floor),
      OPTIMISTIC_TTL_MS,
    )
    pressCar(elevatorId, floor).catch((err) => {
      console.error('pressCar', err)
      window.clearTimeout(timer)
      removeOptimisticCar(elevatorId, floor)
    })
  }

  const onReset = () => {
    setOptimisticHall(new Set())
    setOptimisticCar(new Map())
    setEventLog([])
    resetSimulation().catch((err) => console.error('reset', err))
  }

  return (
    <div className="app">
      <header className="app-header">
        <div className="brand">
          <div className="brand-mark" aria-hidden />
          <div className="brand-text">
            <h1>elevator-go</h1>
            <div className="brand-sub">Operation Simulator</div>
          </div>
        </div>
        <div className="tick">
          <span className="tick-dot" aria-hidden />
          <span className="tick-label">TICK</span>
          <span className="tick-value">{state?.tick ?? '—'}</span>
        </div>
        <a className="header-link" href="/docs" target="_blank" rel="noreferrer">
          API Docs
        </a>
        <button className="reset" onClick={onReset}>
          Reset
        </button>
      </header>
      {state ? (
        <>
          <Building
            state={state}
            optimisticHall={optimisticHall}
            optimisticCar={optimisticCar}
            onHallPress={onHallPress}
            onCarPress={onCarPress}
          />
          <EventLog entries={eventLog} />
        </>
      ) : (
        <p className="loading">接続中…</p>
      )}
    </div>
  )
}

// 区切りは ':' 固定。floor 先頭・'-' 区切りだと負数階（地下階）で split が壊れる。
export const hallKey = (floor: number, direction: 'up' | 'down') => `${direction}:${floor}`
