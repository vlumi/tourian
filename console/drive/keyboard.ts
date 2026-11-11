// W/S or the up and down arrows drive, A/D or the left and right arrows turn. Positive angular is counter-clockwise, as in ROS.
const bindings: Record<string, { linear?: number; angular?: number }> = {
  KeyW: { linear: 1 },
  ArrowUp: { linear: 1 },
  KeyS: { linear: -1 },
  ArrowDown: { linear: -1 },
  KeyA: { angular: 1 },
  ArrowLeft: { angular: 1 },
  KeyD: { angular: -1 },
  ArrowRight: { angular: -1 },
}

function isDriveKey(code: string): boolean {
  return code in bindings
}

export function twistFromKeys(held: Iterable<string>): { linear: number; angular: number } {
  let linear = 0
  let angular = 0
  for (const code of held) {
    linear += bindings[code]?.linear ?? 0
    angular += bindings[code]?.angular ?? 0
  }
  return { linear: Math.sign(linear), angular: Math.sign(angular) }
}

export function watchKeyboard(target: Window = window): { held: Set<string>; stop: () => void } {
  const held = new Set<string>()
  const down = (event: KeyboardEvent) => {
    if (isDriveKey(event.code)) {
      held.add(event.code)
      event.preventDefault()
    }
  }
  const up = (event: KeyboardEvent) => held.delete(event.code)
  const clear = () => held.clear()
  target.addEventListener('keydown', down)
  target.addEventListener('keyup', up)
  target.addEventListener('blur', clear)
  return {
    held,
    stop: () => {
      target.removeEventListener('keydown', down)
      target.removeEventListener('keyup', up)
      target.removeEventListener('blur', clear)
    },
  }
}
