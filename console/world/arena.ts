// The arena the simulated robots drive in, as the console knows it: the square the simulator clamps to, and the furniture that gives a camera something to see. The simulator knows nothing of the furniture; the robot drives through it.
export const arenaHalf = 5

export type Landmark = { x: number; y: number; size: number; height: number; color: string }

export const landmarks: Landmark[] = [
  { x: 3, y: 2, size: 0.8, height: 1.2, color: '#8fd400' },
  { x: -2.5, y: 3, size: 1, height: 0.6, color: '#ff9f36' },
  { x: 2, y: -3, size: 0.6, height: 2, color: '#36a2ff' },
  { x: -3.5, y: -2.5, size: 1.2, height: 0.4, color: '#ff5a36' },
  { x: 0, y: 4.2, size: 0.4, height: 2.5, color: '#e6f5c8' },
]
