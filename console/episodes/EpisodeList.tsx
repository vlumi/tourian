'use client'

import type { Episode } from '@tourian/contract/tourian/v1/episode_pb'
import Link from 'next/link'
import { useEffect, useState } from 'react'
import { errorMessage } from '@/platform/error'
import { listEpisodes } from './api'
import { flag, lasted, started } from './format'

export function EpisodeList() {
  const [episodes, setEpisodes] = useState<Episode[]>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    listEpisodes()
      .then(setEpisodes)
      .catch((e) => setError(errorMessage(e)))
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
                <td>{lasted(episode)}</td>
                <td>{String(episode.frames)}</td>
                <td>{String(episode.videoFrames)}</td>
                <td>{flag(episode)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  )
}
