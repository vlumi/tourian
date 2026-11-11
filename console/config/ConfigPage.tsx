'use client'

import { create } from '@bufbuild/protobuf'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import {
  RobotConfigSchema,
  TaskConfigSchema,
  type RobotConfig,
  type TaskConfig,
} from '@tourian/contract/tourian/v1/config_pb'
import Link from 'next/link'
import { useEffect, useState, type FormEvent } from 'react'
import { connect } from '@/platform/connection'
import { errorMessage } from '@/platform/error'
import { useStore } from '@/platform/store'
import {
  formatParameters,
  listRobotConfigs,
  listTaskConfigs,
  parseParameters,
  saveRobotConfig,
  saveTaskConfig,
} from './api'

// Robots and tasks as the operators set them up; every save is a new version, and the forms are keyed by it so they show what was saved.
export function ConfigPage() {
  const robots = useStore((s) => s.robots)
  const [robotConfigs, setRobotConfigs] = useState<RobotConfig[]>([])
  const [tasks, setTasks] = useState<TaskConfig[]>([])
  const [error, setError] = useState<string>()

  const load = () =>
    Promise.all([listRobotConfigs(), listTaskConfigs()])
      .then(([r, t]) => {
        setRobotConfigs(r)
        setTasks(t)
      })
      .catch((e) => setError(errorMessage(e)))
  useEffect(() => {
    const disconnect = connect()
    void load()
    return disconnect
  }, [])

  const robotIds = Array.from(new Set([...Object.keys(robots), ...robotConfigs.map((c) => c.robotId)])).sort()

  return (
    <main className="config">
      <header>
        <h1>
          <Link href="/">tourian</Link> · configuration
        </h1>
      </header>
      {error && <p className="error">{error}</p>}
      <section>
        <h2>Robots</h2>
        {robotIds.length === 0 && <p className="video-empty">No robots yet.</p>}
        {robotIds.map((id) => (
          <RobotForm
            key={id}
            robotId={id}
            current={robotConfigs.find((c) => c.robotId === id)}
            onSaved={load}
            onError={setError}
          />
        ))}
      </section>
      <section>
        <h2>Tasks</h2>
        {tasks.map((task) => (
          <TaskForm key={`${task.taskId}/${task.version}`} current={task} onSaved={load} onError={setError} />
        ))}
        <TaskForm onSaved={load} onError={setError} />
      </section>
    </main>
  )
}

type Callbacks = { onSaved: () => void; onError: (message: string) => void }

function RobotForm({ robotId, current, onSaved, onError }: { robotId: string; current?: RobotConfig } & Callbacks) {
  const capabilities = useStore((s) => s.robots[robotId])
  const [name, setName] = useState(current?.name ?? '')
  const [maxLinear, setMaxLinear] = useState(current?.limits?.maxLinear ? String(current.limits.maxLinear) : '')
  const [maxAngular, setMaxAngular] = useState(current?.limits?.maxAngular ? String(current.limits.maxAngular) : '')
  const [parameters, setParameters] = useState(formatParameters(current?.parameters ?? {}))
  const [note, setNote] = useState(current?.note ?? '')

  const submit = (event: FormEvent) => {
    event.preventDefault()
    saveRobotConfig(
      create(RobotConfigSchema, {
        robotId,
        name,
        limits: { maxLinear: Number(maxLinear) || 0, maxAngular: Number(maxAngular) || 0 },
        parameters: parseParameters(parameters),
        note,
      }),
    )
      .then(onSaved)
      .catch((e) => onError(errorMessage(e)))
  }

  return (
    <form className="config-form" onSubmit={submit}>
      <h3>
        {robotId}
        <span className="version">
          {current ? `v${current.version} · ${when(current.updatedAt)}` : 'not configured'}
        </span>
      </h3>
      <label>
        name
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder={capabilities?.name} />
      </label>
      <label>
        max linear, m/s
        <input
          value={maxLinear}
          onChange={(e) => setMaxLinear(e.target.value)}
          placeholder={`robot: ${capabilities?.velocityLimits?.maxLinear ?? '?'}`}
        />
      </label>
      <label>
        max angular, rad/s
        <input
          value={maxAngular}
          onChange={(e) => setMaxAngular(e.target.value)}
          placeholder={`robot: ${capabilities?.velocityLimits?.maxAngular ?? '?'}`}
        />
      </label>
      <label>
        parameters, one per line
        <textarea
          value={parameters}
          onChange={(e) => setParameters(e.target.value)}
          rows={3}
          placeholder="key = value"
        />
      </label>
      <label>
        note
        <input value={note} onChange={(e) => setNote(e.target.value)} />
      </label>
      <button type="submit">Save as v{(current?.version ?? 0) + 1}</button>
    </form>
  )
}

function TaskForm({ current, onSaved, onError }: { current?: TaskConfig } & Callbacks) {
  const [taskId, setTaskId] = useState(current?.taskId ?? '')
  const [name, setName] = useState(current?.name ?? '')
  const [description, setDescription] = useState(current?.description ?? '')
  const [success, setSuccess] = useState(current?.success ?? '')

  const submit = (event: FormEvent) => {
    event.preventDefault()
    saveTaskConfig(create(TaskConfigSchema, { taskId, name, description, success }))
      .then(() => {
        if (!current) {
          setTaskId('')
          setName('')
          setDescription('')
          setSuccess('')
        }
        onSaved()
      })
      .catch((e) => onError(errorMessage(e)))
  }

  return (
    <form className="config-form" onSubmit={submit}>
      <h3>
        {current ? current.taskId : 'New task'}
        <span className="version">{current ? `v${current.version} · ${when(current.updatedAt)}` : ''}</span>
      </h3>
      {!current && (
        <label>
          id
          <input value={taskId} onChange={(e) => setTaskId(e.target.value)} placeholder="drive-square" required />
        </label>
      )}
      <label>
        name
        <input value={name} onChange={(e) => setName(e.target.value)} required />
      </label>
      <label>
        description
        <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
      </label>
      <label>
        success looks like
        <input value={success} onChange={(e) => setSuccess(e.target.value)} />
      </label>
      <button type="submit">{current ? `Save as v${current.version + 1}` : 'Create'}</button>
    </form>
  )
}

function when(stamp: TaskConfig['updatedAt']): string {
  return stamp ? timestampDate(stamp).toLocaleString() : ''
}
