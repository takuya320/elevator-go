import type { Elevator, FloorRange } from '../types'
import { hallKey } from '../App'

export function Floor({
  floor,
  range,
  elevators,
  optimisticHall,
  onHallPress,
}: {
  floor: number
  range: FloorRange
  elevators: Elevator[]
  optimisticHall: Set<string>
  onHallPress: (floor: number, direction: 'up' | 'down') => void
}) {
  const upDisabled = floor === range.max
  const downDisabled = floor === range.min

  // サーバ状態 (assignedHallCalls) または optimistic で点灯。
  const upLit =
    optimisticHall.has(hallKey(floor, 'up')) ||
    elevators.some((e) =>
      e.assignedHallCalls.some((c) => c.floor === floor && c.direction === 'up'),
    )
  const downLit =
    optimisticHall.has(hallKey(floor, 'down')) ||
    elevators.some((e) =>
      e.assignedHallCalls.some((c) => c.floor === floor && c.direction === 'down'),
    )

  return (
    <div className="floor">
      <div className="floor-num">{floor}</div>
      <div className="hall-buttons">
        <button
          className={upLit ? 'lit' : ''}
          disabled={upDisabled}
          onClick={() => onHallPress(floor, 'up')}
        >
          ▲
        </button>
        <button
          className={downLit ? 'lit' : ''}
          disabled={downDisabled}
          onClick={() => onHallPress(floor, 'down')}
        >
          ▼
        </button>
      </div>
      <div className="shafts">
        {elevators.map((e) => (
          <Shaft key={e.id} floor={floor} elevator={e} />
        ))}
      </div>
    </div>
  )
}

function Shaft({ floor, elevator }: { floor: number; elevator: Elevator }) {
  const here = elevator.currentFloor === floor
  const open = elevator.doorState === 'open'
  const unavailable = elevator.operationState !== 'running'
  const shaftCls = ['shaft']
  if (unavailable) shaftCls.push('unavailable')

  const carCls = ['car']
  if (open) carCls.push('door-open')
  if (unavailable) carCls.push('disabled')

  return (
    <div className={shaftCls.join(' ')}>
      {here && (
        <div className={carCls.join(' ')} aria-label={open ? '扉開' : '扉閉'}>
          <div className="car-icon" aria-hidden />
          <div className="car-id">{elevator.id}</div>
        </div>
      )}
    </div>
  )
}
