'use client'

import { useEffect, useState } from 'react'
import { platformHttp } from '@/platform/http'

type Addresses = { lan: string; secure?: string }

// Over plain http a browser hands out no camera, so this page, reached without any warning, finds the platform's https address and goes there if the phone already trusts it; if not, it walks the phone through trusting it once.
export function FirstVisit() {
  const [secure, setSecure] = useState<string>()
  const [trusted, setTrusted] = useState<boolean>()

  useEffect(() => {
    let cancelled = false
    fetch(platformHttp() + '/addresses')
      .then((response) => response.json() as Promise<Addresses>)
      .then(({ secure }) => {
        if (cancelled || !secure) return
        const there = secure + window.location.pathname + window.location.search
        setSecure(there)
        return fetch(secure + '/api/healthz', { mode: 'cors' })
          .then(() => window.location.replace(there))
          .catch(() => !cancelled && setTrusted(false))
      })
      .catch(() => !cancelled && setTrusted(false))
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <main className="camera">
      <h1>tourian camera</h1>
      {trusted === undefined && <p className="hint">Looking for the platform's secure address…</p>}
      {trusted === false && !secure && (
        <p className="error">The platform serves no https, and a browser lends its camera only over https.</p>
      )}
      {trusted === false && secure && (
        <>
          <p>This phone must trust the platform once; the camera is handed out only over https.</p>
          <ol className="steps">
            <li>
              <a href={platformHttp() + '/ca.pem'}>Install the certificate</a>. On iOS, open Settings, install the
              downloaded profile, then turn it on under General › About › Certificate Trust Settings. On Android,
              install it as a CA certificate under Settings › Security.
            </li>
            <li>
              <a href={secure}>Continue to the camera</a>.
            </li>
          </ol>
          <p className="hint">From then on, scanning the code goes straight to the camera.</p>
        </>
      )}
    </main>
  )
}
