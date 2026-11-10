import { fromJson, fromJsonString } from '@bufbuild/protobuf'
import {
  EpisodeFrameSchema,
  EpisodeSchema,
  EpisodeVideoSchema,
  type Episode,
  type EpisodeFrame,
  type EpisodeVideo,
} from '@tourian/contract/tourian/v1/episode_pb'
import { platformHttp } from '@/platform/socket'

export async function listEpisodes(): Promise<Episode[]> {
  const response = await fetch(`${platformHttp()}/episodes`)
  const items = (await response.json()) as unknown[]
  return items.map((item) => fromJson(EpisodeSchema, item as Parameters<typeof fromJson>[1]))
}

export type Recording = { episode: Episode; frames: EpisodeFrame[]; video: EpisodeVideo[] }

export async function loadEpisode(id: string): Promise<Recording> {
  const base = `${platformHttp()}/episodes/${encodeURIComponent(id)}`
  const [meta, frames, video] = await Promise.all([
    fetch(base).then((r) => r.text()),
    fetch(`${base}/frames`).then((r) => r.text()),
    fetch(`${base}/video`).then((r) => r.text()),
  ])
  return {
    episode: fromJsonString(EpisodeSchema, meta),
    frames: parseLines(frames).map((line) => fromJsonString(EpisodeFrameSchema, line)),
    video: parseLines(video).map((line) => fromJsonString(EpisodeVideoSchema, line)),
  }
}

export function pictureUrl(id: string, picture: EpisodeVideo): string {
  return `${platformHttp()}/episodes/${encodeURIComponent(id)}/${picture.file}`
}

export function parseLines(text: string): string[] {
  return text.split('\n').filter((line) => line.trim() !== '')
}
