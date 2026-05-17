// API 呼び出しは openapi-fetch を経由して、URL・パスパラメータ・リクエスト body・
// レスポンス型まで全て schema.d.ts に追従させる。
// 仕様変更時は `pnpm run gen:api`（または build）で schema.d.ts を更新すれば
// 型エラーが出る箇所が自動的に集約される。

import createClient from 'openapi-fetch'
import type { paths } from './api/schema'
import type { OperationState } from './types'

// VITE_API_BASE が設定されていれば別オリジン (例: backend を別ホストにデプロイした場合) に向ける。
// 未設定なら同一オリジンを叩く (embed 配信 / Vite dev server proxy 用)。
const baseUrl = import.meta.env.VITE_API_BASE?.trim() || '/'
const client = createClient<paths>({ baseUrl })

function unwrap<T>(data: T | undefined, error: unknown): T {
  if (error !== undefined) throw new Error(JSON.stringify(error))
  if (data === undefined) throw new Error('empty response')
  return data
}

export async function pressHall(floor: number, direction: 'up' | 'down') {
  const { data, error } = await client.POST('/floors/{floor}/hall-calls', {
    params: { path: { floor } },
    body: { direction },
  })
  return unwrap(data, error)
}

export async function pressCar(elevatorId: string, destinationFloor: number) {
  const { data, error } = await client.POST('/elevators/{elevatorId}/car-calls', {
    params: { path: { elevatorId } },
    body: { destinationFloor },
  })
  return unwrap(data, error)
}

export async function resetSimulation() {
  const { data, error } = await client.POST('/simulation/reset', { body: {} })
  return unwrap(data, error)
}

export async function setOperationState(elevatorId: string, state: OperationState) {
  // running は POST /resume、stopped は POST /stop が用意されているが、
  // maintenance は PATCH しか手段が無いので統一して PATCH を使う。
  const { data, error } = await client.PATCH('/elevators/{elevatorId}', {
    params: { path: { elevatorId } },
    body: { operationState: state },
  })
  return unwrap(data, error)
}

export async function openDoor(elevatorId: string) {
  const { data, error } = await client.POST('/elevators/{elevatorId}/doors/open', {
    params: { path: { elevatorId } },
  })
  return unwrap(data, error)
}

export async function closeDoor(elevatorId: string) {
  const { data, error } = await client.POST('/elevators/{elevatorId}/doors/close', {
    params: { path: { elevatorId } },
  })
  return unwrap(data, error)
}

export async function cancelHallCall(callId: string) {
  // 204 No Content。data は undefined になる。
  const { error } = await client.DELETE('/hall-calls/{callId}', {
    params: { path: { callId } },
  })
  if (error !== undefined) throw new Error(JSON.stringify(error))
}

export async function setHomeFloor(elevatorId: string, homeFloor: number) {
  const { data, error } = await client.PATCH('/elevators/{elevatorId}', {
    params: { path: { elevatorId } },
    body: { homeFloor },
  })
  return unwrap(data, error)
}

export async function setAutoReturnEnabled(elevatorId: string, enabled: boolean) {
  const { data, error } = await client.PATCH('/elevators/{elevatorId}', {
    params: { path: { elevatorId } },
    body: { autoReturnEnabled: enabled },
  })
  return unwrap(data, error)
}
