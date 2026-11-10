import { Suspense } from 'react'
import { ConfigPage } from '@/config/ConfigPage'

export default function Page() {
  return (
    <Suspense>
      <ConfigPage />
    </Suspense>
  )
}
