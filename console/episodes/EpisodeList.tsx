'use client'

import { timestampDate } from '@bufbuild/protobuf/wkt'
import type { Episode } from '@tourian/contract/tourian/v1/episode_pb'
import Link from 'next/link'
import { useEffect, useState } from 'react'
import { listEpisodes } from './api'

export function EpisodeList() {
  const [episodes, setEpisodes] = useState<Episode[]>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    listEpisodes()
      .then(setEpisodes)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])

  return (
    <main className="episodes">
      <header>
        <h1>
          <Link href="/">tourian</Link> · episodes
        </h1>
      </header>
      {error && <p className="error">{error}</p>}
      {episodes && episodes.length === 0 && (
        <p className="video-empty">No episodes yet. Record one from the console.</p>
      )}
      {episodes && episodes.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Started</th>
              <th>Robot</th>
              <th>Task</th>
              <th>Length</th>
              <th>Frames</th>
              <th>Pictures</th>
              <th>Flag</th>
            </tr>
          </thead>
          <tbody>
            {episodes.map((episode) => (
              <tr key={episode.id} className={episode.bad ? 'bad' : ''}>
                <td>
                  <Link href={`/episodes/${encodeURIComponent(episode.id)}`}>{started(episode)}</Link>
                </td>
                <td>
                  {episode.robotId}
                  {episode.robotConfigVersion > 0 && <span className="version"> v{episode.robotConfigVersion}</span>}
                </td>
                <td>
                  {episode.task || '–'}
                  {episode.taskConfigVersion > 0 && <span className="version"> v{episode.taskConfigVersion}</span>}
                </td>
                <td>{length(episode)}</td>
                <td>{String(episode.frames)}</td>
                <td>{String(episode.videoFrames)}</td>
                <td>{episode.bad ? `bad${episode.note ? `: ${episode.note}` : ''}` : ''}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  )
}

export function started(episode: Episode): string {
  return episode.startedAt ? timestampDate(episode.startedAt).toLocaleString() : episode.id
}

export function length(episode: Episode): string {
  if (!episode.startedAt || !episode.endedAt) return '–'
  const seconds = (timestampDate(episode.endedAt).getTime() - timestampDate(episode.startedAt).getTime()) / 1000
  return `${seconds.toFixed(1)} s`
}
