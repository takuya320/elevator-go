import type { SimulationEvent, OperationState } from '../types'

// id は App 側で振る表示用の連番。イベント自体は一意なキーを持たない。
export type LogEntry = { id: number; event: SimulationEvent }

export function EventLog({ entries }: { entries: LogEntry[] }) {
  return (
    <div className="event-log">
      <div className="event-log-header">
        <h3>Activity</h3>
        <span className="event-log-count">{entries.length} events</span>
      </div>
      {entries.length === 0 ? (
        <p className="event-log-empty">まだイベントがありません</p>
      ) : (
        <ol className="event-log-list">
          {entries.map(({ id, event: e }) => (
            <li key={id}>
              <time>{formatTime(e.timestamp)}</time>
              <span className={`event-tag ${tagClass(e)}`}>{tagLabel(e)}</span>
              <span className="event-msg">{describe(e)}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

function tagClass(e: SimulationEvent): string {
  switch (e.type) {
    case 'hall_call.requested':
    case 'hall_call.served':
    case 'hall_call.canceled':
      return 'tag-hall'
    case 'car_call.requested':
      return 'tag-car'
    case 'elevator.arrived':
      return 'tag-arr'
    case 'elevator.state_changed':
      return 'tag-state'
  }
}

function tagLabel(e: SimulationEvent): string {
  switch (e.type) {
    case 'hall_call.requested': return 'HALL'
    case 'hall_call.served':    return 'SERVED'
    case 'hall_call.canceled':  return 'CANCEL'
    case 'car_call.requested':  return 'CAR'
    case 'elevator.arrived':    return 'ARRIVE'
    case 'elevator.state_changed': return 'STATE'
  }
}

function describe(e: SimulationEvent): string {
  switch (e.type) {
    case 'hall_call.requested':
      return `${e.floor}F ${arrow(e.direction)} → ${e.elevatorId}`
    case 'hall_call.served':
      return `${e.elevatorId} が ${e.floor}F の呼びを完了`
    case 'hall_call.canceled':
      return `呼び ${shortId(e.callId)} をキャンセル`
    case 'car_call.requested':
      return `${e.elevatorId} → ${e.floor}F`
    case 'elevator.arrived':
      return `${e.elevatorId} が ${e.floor}F に到着・開扉`
    case 'elevator.state_changed':
      return `${e.elevatorId}: ${labelState(e.from)} → ${labelState(e.to)}`
  }
}

function arrow(d?: 'up' | 'down') {
  return d === 'up' ? '↑' : d === 'down' ? '↓' : ''
}

function shortId(id?: string) {
  if (!id) return ''
  return id.slice(0, 8)
}

function labelState(s?: OperationState) {
  return s === 'running' ? '運転' : s === 'stopped' ? '停止' : s === 'maintenance' ? '点検' : (s ?? '')
}

function formatTime(iso: string) {
  const d = new Date(iso)
  return d.toLocaleTimeString('ja-JP', { hour12: false })
}
