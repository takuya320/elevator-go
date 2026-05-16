import type { Elevator, OperationState } from '../types'
import {
  closeDoor,
  openDoor,
  setAutoReturnEnabled,
  setHomeFloor,
  setOperationState,
} from '../api'

export function CarControls({
  elevator,
  optimisticFloors,
  onCarPress,
}: {
  elevator: Elevator
  optimisticFloors: Set<number>
  onCarPress: (elevatorId: string, floor: number) => void
}) {
  const range = elevator.floorRange
  const floors: number[] = []
  for (let f = range.max; f >= range.min; f--) floors.push(f)

  const lit = new Set(elevator.destinationFloors)
  const disabled = elevator.operationState !== 'running'

  return (
    <div className={`car-controls ${disabled ? 'is-disabled' : ''}`}>
      <div className="car-controls-header">
        <h3>{elevator.id}</h3>
        <div>
          <span className="floor-now-label">FLOOR</span>
          <span className="floor-now">{elevator.currentFloor}</span>
        </div>
      </div>
      <div className="meta">
        <span className={`meta-pill direction-${elevator.direction}`}>{labelDirection(elevator.direction)}</span>
        <span className={`meta-pill door-${elevator.doorState}`}>{labelDoor(elevator.doorState)}</span>
        <span className={`meta-pill op-${elevator.operationState}`}>{labelOp(elevator.operationState)}</span>
      </div>
      <div className="car-buttons">
        {floors.map((f) => (
          <button
            key={f}
            className={lit.has(f) || optimisticFloors.has(f) ? 'lit' : ''}
            disabled={disabled}
            onClick={() => onCarPress(elevator.id, f)}
          >
            {f}
          </button>
        ))}
      </div>
      <DoorControls elevator={elevator} />
      <AdminControls elevator={elevator} />
      <HomeControls elevator={elevator} />
    </div>
  )
}

function HomeControls({ elevator }: { elevator: Elevator }) {
  const range = elevator.floorRange
  const floors: number[] = []
  for (let f = range.max; f >= range.min; f--) floors.push(f)
  return (
    <div className="home-controls">
      <span className="home-label">自動帰還</span>
      <div className="home-row">
        <label className="home-toggle">
          <input
            type="checkbox"
            checked={elevator.autoReturnEnabled}
            onChange={(ev) =>
              setAutoReturnEnabled(elevator.id, ev.target.checked).catch((err) =>
                console.error('setAutoReturnEnabled', err),
              )
            }
          />
          有効化
        </label>
        <label className="home-select">
          ホーム階
          <select
            value={elevator.homeFloor}
            onChange={(ev) =>
              setHomeFloor(elevator.id, Number(ev.target.value)).catch((err) =>
                console.error('setHomeFloor', err),
              )
            }
          >
            {floors.map((f) => (
              <option key={f} value={f}>
                {f}
              </option>
            ))}
          </select>
        </label>
      </div>
    </div>
  )
}

function DoorControls({ elevator }: { elevator: Elevator }) {
  return (
    <div className="door-controls">
      <span className="door-label">扉</span>
      <div className="door-buttons">
        <button
          className={elevator.doorHoldOpen ? 'active' : ''}
          onClick={() => openDoor(elevator.id).catch((err) => console.error('openDoor', err))}
          aria-label="開"
        >
          開
        </button>
        <button
          onClick={() => closeDoor(elevator.id).catch((err) => console.error('closeDoor', err))}
          aria-label="閉"
        >
          閉
        </button>
      </div>
    </div>
  )
}

const STATES: { state: OperationState; label: string }[] = [
  { state: 'running', label: '運転' },
  { state: 'stopped', label: '停止' },
  { state: 'maintenance', label: '点検' },
]

function AdminControls({ elevator }: { elevator: Elevator }) {
  return (
    <div className="admin-controls">
      <span className="admin-label">管理</span>
      <div className="admin-buttons">
        {STATES.map(({ state, label }) => (
          <button
            key={state}
            className={elevator.operationState === state ? 'active' : ''}
            disabled={elevator.operationState === state}
            onClick={() =>
              setOperationState(elevator.id, state).catch((err) => console.error('setState', err))
            }
          >
            {label}
          </button>
        ))}
      </div>
    </div>
  )
}

function labelDirection(d: Elevator['direction']) {
  return d === 'up' ? '上昇' : d === 'down' ? '下降' : '待機'
}
function labelDoor(d: Elevator['doorState']) {
  return d === 'open' ? '扉開' : '扉閉'
}
function labelOp(s: Elevator['operationState']) {
  return s === 'running' ? '運転' : s === 'stopped' ? '停止' : '点検'
}
