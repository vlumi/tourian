import { timestampDate } from '@bufbuild/protobuf/wkt'
import type { Episode } from '@tourian/contract/tourian/v1/episode_pb'

export function started(episode: Episode): string {
  return episode.startedAt ? timestampDate(episode.startedAt).toLocaleString() : episode.id
}

export function lasted(episode: Episode): string {
  if (!episode.startedAt || !episode.endedAt) return '–'
  const seconds = (timestampDate(episode.endedAt).getTime() - timestampDate(episode.startedAt).getTime()) / 1000
  return `${seconds.toFixed(1)} s`
}

export function flag(episode: Episode): string {
  return episode.bad ? `bad${episode.note ? `: ${episode.note}` : ''}` : ''
}
