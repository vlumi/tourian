import type { ReactNode } from 'react'

export function Row({ label, children, spark }: { label: string; children: ReactNode; spark?: ReactNode }) {
  return (
    <div className="row">
      <span className="label">{label}</span>
      {spark}
      <span className="value">{children}</span>
    </div>
  )
}
