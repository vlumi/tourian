import { fromJson, toJson } from '@bufbuild/protobuf'
import {
  RobotConfigSchema,
  TaskConfigSchema,
  type RobotConfig,
  type TaskConfig,
} from '@tourian/contract/tourian/v1/config_pb'
import { platformHttp } from '@/platform/socket'

export async function listRobotConfigs(): Promise<RobotConfig[]> {
  return parseAll(RobotConfigSchema, await getJson('/config/robots'))
}

export async function listTaskConfigs(): Promise<TaskConfig[]> {
  return parseAll(TaskConfigSchema, await getJson('/config/tasks'))
}

export async function saveRobotConfig(config: RobotConfig): Promise<RobotConfig> {
  const body = await put(`/config/robots/${encodeURIComponent(config.robotId)}`, toJson(RobotConfigSchema, config))
  return fromJson(RobotConfigSchema, body as Parameters<typeof fromJson>[1])
}

export async function saveTaskConfig(config: TaskConfig): Promise<TaskConfig> {
  const body = await put(`/config/tasks/${encodeURIComponent(config.taskId)}`, toJson(TaskConfigSchema, config))
  return fromJson(TaskConfigSchema, body as Parameters<typeof fromJson>[1])
}

async function getJson(path: string): Promise<unknown[]> {
  const response = await fetch(platformHttp() + path)
  if (!response.ok) throw new Error(`${path}: ${response.status}`)
  return (await response.json()) as unknown[]
}

async function put(path: string, body: unknown): Promise<unknown> {
  const response = await fetch(platformHttp() + path, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!response.ok) throw new Error(`${path}: ${response.status} ${await response.text()}`)
  return response.json()
}

function parseAll<T>(schema: Parameters<typeof fromJson>[0], items: unknown[]): T[] {
  return items.map((item) => fromJson(schema, item as Parameters<typeof fromJson>[1]) as T)
}

// Parameters are edited as one `key = value` per line.
export function parseParameters(text: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const line of text.split('\n')) {
    const at = line.indexOf('=')
    if (at < 0) continue
    const key = line.slice(0, at).trim()
    if (key) out[key] = line.slice(at + 1).trim()
  }
  return out
}

export function formatParameters(parameters: Record<string, string>): string {
  return Object.entries(parameters)
    .map(([key, value]) => `${key} = ${value}`)
    .join('\n')
}
