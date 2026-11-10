'use client'

import { create } from '@bufbuild/protobuf'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { EpisodeFlagSchema, RecordRequestSchema } from '@tourian/contract/tourian/v1/episode_pb'
import Link from 'next/link'
import { useEffect, useState } from 'react'
import { send } from '@/platform/connection'
import { useStore } from '@/platform/store'

// Start and stop recording the selected robot, see what is being recorded, and mark the latest episode bad while it is fresh.
export function RecordControls() {
  const robotId = useStore((s) => s.selected)
  const episode = useStore((s) => s.episode)
  const bad = useStore((s) => s.episodeBad)
  const setEpisodeBad = useStore((s) => s.setEpisodeBad)
  const [task, setTask] = useState('')
  const [elapsed, setElapsed] = useState(0)

  const recording = episode?.recording ?? false
  useEffect(() => {
    if (!recording || !episode?.startedAt) return
    const started = timestampDate(episode.startedAt).getTime()
    const tick = () => setElapsed(Math.floor((Date.now() - started) / 1000))
    tick()
    const timer = setInterval(tick, 250)
    return () => clearInterval(timer)
  }, [recording, episode?.startedAt])

  const toggle = () => {
    if (!robotId) return
    send({ case: 'recordRequest', value: create(RecordRequestSchema, { robotId, start: !recording, task }) })
  }
  const flag = () => {
    if (!episode) return
    const next = !bad
    const note = next ? (window.prompt('Why is this episode bad?', '') ?? '') : ''
    send({ case: 'episodeFlag', value: create(EpisodeFlagSchema, { episodeId: episode.episodeId, bad: next, note }) })
    setEpisodeBad(next)
  }

  return (
    <div className="record">
      <input
        value={task}
        onChange={(e) => setTask(e.target.value)}
        placeholder="task"
        disabled={recording}
        aria-label="Task being recorded"
      />
      <button type="button" onClick={toggle} disabled={!robotId} className={recording ? 'recording' : ''}>
        {recording ? `Stop · ${format(elapsed)}` : 'Record'}
      </button>
      {episode && (
        <span className="episode">
          <span className="label">{recording ? 'rec' : 'last'}</span> {String(episode.frames)} frames ·{' '}
          {String(episode.videoFrames)} pictures
          <button type="button" onClick={flag} className={bad ? 'bad' : ''}>
            {bad ? 'Bad' : 'Mark bad'}
          </button>
        </span>
      )}
      <Link href="/episodes">Episodes</Link>
    </div>
  )
}

function format(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`
}
