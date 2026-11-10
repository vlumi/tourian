# Screenshots

One capture per milestone, taken with `make screenshot` (which builds and serves the console and runs [`scripts/screenshot.mjs`](../../scripts/screenshot.mjs) against it), newest first. The files carry a running number so they sort in order, then a name for what they show; the headings carry the dates.

## 2026-11-10 · Drive

![Driving the simulated robot](001-drive.png)

The console engaged and driving the simulated robot over WebSocket: the arena with its meter grid and the trail of a left turn followed by a straight run, the telemetry panel with the link, the robot's pose, speed, turn rate, battery and wheel speeds, sparklines beside the figures, and the round-trip latency of each command measured from its acknowledgement, under a millisecond on the same machine. Taken with the platform running the simulator in-process and the keys scripted.

## 2026-11-10 · Bootstrap

![The console after M0](000-bootstrap.png)

The console's first page: the name, the tagline and the seven messages of the first contract, read from the generated TypeScript schemas at render time.
