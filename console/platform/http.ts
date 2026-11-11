import { fromJson, type DescMessage, type JsonValue, type MessageShape } from '@bufbuild/protobuf'
import { platformBase } from './socket'

// The same platform over HTTP, for what is fetched rather than streamed.
export function platformHttp(): string {
  return platformBase().replace(/^ws/, 'http')
}

export async function getJson(path: string): Promise<JsonValue> {
  const response = await fetch(platformHttp() + path)
  if (!response.ok) throw new Error(`${path}: ${response.status}`)
  return response.json()
}

export async function putJson(path: string, body: JsonValue): Promise<JsonValue> {
  const response = await fetch(platformHttp() + path, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!response.ok) throw new Error(`${path}: ${response.status} ${await response.text()}`)
  return response.json()
}

export function parseAll<Desc extends DescMessage>(schema: Desc, items: JsonValue): MessageShape<Desc>[] {
  if (!Array.isArray(items)) throw new Error(`expected a list, got ${typeof items}`)
  return items.map((item) => fromJson(schema, item))
}
