import type { Input } from '@/platform/store'
import type { GamepadReading } from './gamepad'

// A moved stick drives; otherwise the keys do, and a connected but idle gamepad is still named as the source.
export function pickInput(pad: GamepadReading, keys: { linear: number; angular: number }): Input {
  if (pad.connected && (pad.linear !== 0 || pad.angular !== 0)) {
    return { linear: pad.linear, angular: pad.angular, source: 'gamepad' }
  }
  if (keys.linear || keys.angular) return { ...keys, source: 'keyboard' }
  return { ...keys, source: pad.connected ? 'gamepad' : 'none' }
}
