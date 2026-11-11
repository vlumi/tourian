import { fromJsonString } from '@bufbuild/protobuf'
import {
  EpisodeFrameSchema,
  EpisodeSchema,
  EpisodeVideoSchema,
  type Episode,
  type EpisodeFrame,
  type EpisodeVideo,
} from '@tourian/contract/tourian/v1/episode_pb'
import { getJson, parseAll, platformHttp } from '@/platform/http'

export async function listEpisodes(): Promise<Episode[]> {
  return parseAll(EpisodeSchema, await getJson('/episodes'))
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
