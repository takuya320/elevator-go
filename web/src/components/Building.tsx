import type { SimulationState } from '../types'
import { Floor } from './Floor'
import { CarControls } from './CarControls'

export function Building({
  state,
  optimisticHall,
  optimisticCar,
  onHallPress,
  onCarPress,
}: {
  state: SimulationState
  optimisticHall: Set<string>
  optimisticCar: Map<string, Set<number>>
  onHallPress: (floor: number, direction: 'up' | 'down') => void
  onCarPress: (elevatorId: string, floor: number) => void
}) {
  if (state.elevators.length === 0) {
    return <p>エレベーターがありません</p>
  }
  // 全号機の floorRange は同一前提（同じ建物）。先頭から取り出す。
  const range = state.elevators[0].floorRange
  const floors: number[] = []
  for (let f = range.max; f >= range.min; f--) floors.push(f)

  return (
    <div className="building">
      <div className="floors">
        {floors.map((f) => (
          <Floor
            key={f}
            floor={f}
            range={range}
            elevators={state.elevators}
            optimisticHall={optimisticHall}
            onHallPress={onHallPress}
          />
        ))}
      </div>
      <div className="controls">
        {state.elevators.map((e) => (
          <CarControls
            key={e.id}
            elevator={e}
            optimisticFloors={optimisticCar.get(e.id) ?? new Set()}
            onCarPress={onCarPress}
          />
        ))}
      </div>
    </div>
  )
}
