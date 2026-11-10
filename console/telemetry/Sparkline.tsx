type Props = { values: number[]; min?: number; max?: number; width?: number; height?: number }

// The last samples of one value as a line; the range is fixed when given and fitted otherwise.
export function Sparkline({ values, min, max, width = 160, height = 32 }: Props) {
  if (values.length < 2) return <svg className="sparkline" width={width} height={height} aria-hidden />
  const low = min ?? Math.min(...values)
  const high = max ?? Math.max(...values)
  const span = high - low || 1
  const step = width / (values.length - 1)
  const points = values
    .map((v, i) => `${(i * step).toFixed(1)},${(height - 2 - ((v - low) / span) * (height - 4)).toFixed(1)}`)
    .join(' ')
  return (
    <svg className="sparkline" width={width} height={height} aria-hidden>
      <polyline points={points} fill="none" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  )
}
