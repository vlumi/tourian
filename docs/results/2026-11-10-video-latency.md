# Video latency, 2026-11-10

Age of each frame when the console shows it, counted from the moment the camera page composed it, with the camera page and the console in one headless Chrome on the same machine. The age covers encoding, the path, decoding and rendering, not the camera's sensor or the display. Measured by `make results-video` with a fake camera at 640×480 for 10 s per path after a warm-up.

- Commit: `761c77c`
- Machine: Apple M4 Max, macOS 27.2.1
- Browser: Chrome/157.0.8103.39
- Platform and camera page on localhost, the simulated robot in-process

| Path | Frames shown | fps | Median | p95 | Min | Max |
| --- | --- | --- | --- | --- | --- | --- |
| WebRTC, peer to peer | 200 | 20 | 30 ms | 31 ms | 27 ms | 33 ms |
| MJPEG over WebSocket through the platform | 151 | 15.1 | 36 ms | 58 ms | 5 ms | 74 ms |
