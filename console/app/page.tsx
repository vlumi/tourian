import { Suspense } from 'react'
import { ControlRoom } from './ControlRoom'

export default function Page() {
  return (
    <Suspense>
      <ControlRoom />
    </Suspense>
  )
}
