// Round-trip time of each command, from the moment it is sent to the moment its acknowledgement arrives.
const sent = new Map<bigint, number>()
const forgetAfterMs = 5000

export function markSent(sequence: bigint, now = performance.now()): void {
  sent.set(sequence, now)
  for (const [seq, at] of sent) {
    if (now - at > forgetAfterMs) sent.delete(seq)
    else break
  }
}

export function markAcked(sequence: bigint, now = performance.now()): number | undefined {
  const at = sent.get(sequence)
  if (at === undefined) return undefined
  sent.delete(sequence)
  return now - at
}
