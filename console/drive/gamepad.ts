// The left stick drives and the right stick turns; B is the emergency stop, Start toggles engagement.
const deadzone = 0.15
const buttons = { stop: 1, engage: 9 }

export type GamepadReading = {
  connected: boolean
  linear: number
  angular: number
  stopPressed: boolean
  engagePressed: boolean
}

let previous = { stop: false, engage: false }

export function readGamepad(pads: (Gamepad | null)[] = navigator.getGamepads?.() ?? []): GamepadReading {
  const pad = pads.find((p) => p?.connected)
  if (!pad) {
    previous = { stop: false, engage: false }
    return { connected: false, linear: 0, angular: 0, stopPressed: false, engagePressed: false }
  }
  const stop = pad.buttons[buttons.stop]?.pressed ?? false
  const engage = pad.buttons[buttons.engage]?.pressed ?? false
  const reading = {
    connected: true,
    linear: axis(pad.axes[1], -1),
    angular: axis(pad.axes[2] ?? pad.axes[0], -1),
    stopPressed: stop && !previous.stop,
    engagePressed: engage && !previous.engage,
  }
  previous = { stop, engage }
  return reading
}

function axis(value: number | undefined, direction: 1 | -1): number {
  if (value === undefined || Math.abs(value) < deadzone) return 0
  return direction * Math.sign(value) * ((Math.abs(value) - deadzone) / (1 - deadzone))
}
