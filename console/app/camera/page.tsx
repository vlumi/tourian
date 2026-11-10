import { Suspense } from 'react'
import { CameraPage } from './CameraPage'

export default function Page() {
  return (
    <Suspense>
      <CameraPage />
    </Suspense>
  )
}
