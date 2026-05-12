// docs/openapi.yaml から openapi-typescript で生成した schema.d.ts を一次ソースとし、
// アプリ側で使う型はここで再 export する。types.ts に重複定義しない。

import type { components } from './api/schema'

type Schemas = components['schemas']

export type Direction = Schemas['Direction']
export type DoorState = Schemas['DoorState']
export type OperationState = Schemas['OperationState']
export type HallCallStatus = Schemas['HallCallStatus']
export type FloorRange = Schemas['FloorRange']
export type HallCall = Schemas['HallCall']
export type Elevator = Schemas['Elevator']

export type SimulationState = Schemas['SimulationTickResponse']
export type SimulationEvent = Schemas['SimulationEvent']
export type SimulationEventType = Schemas['SimulationEventType']
