# tourian

A control room for robots, and for whatever is at the controls.

Tourian is a teleoperation platform: an operator drives a robot from a browser over a live video feed, every observation and action is recorded as an episode for training, and control of any robot can be handed from the human to a scripted routine or a learned policy and taken back, with a kill switch that works even when nothing else does. The robots start simulated; the seams are placed so that a ROS 2 robot or a physical arm slots in behind the same contract.

It is an experiment for understanding the domain by building it, not a product. The same console drives every robot through one Protobuf contract, with the transport, the video path, the dataset format and the robot itself each swappable, so that technologies can be compared against each other on the same system.

**The name:** Tourian is the sector of planet Zebes where Mother Brain sits. The brain is the controller, whoever holds it; the sector is the platform. The components keep the theme: `motherbrain` for the controller that holds a robot, `emmi` for the robot adapters, `zebes` for the simulated world, `samus` for the operator. Named after locations and characters in Nintendo's Metroid series; not affiliated with or endorsed by Nintendo.

**Status:** bootstrapped 2026-11-10; the simulated robot drives from the console over WebSocket with the safety behaviors in the robot (M1), and a phone's camera shows on the console over WebRTC or MJPEG, both paths measured (M2). [SCOPE.md](SCOPE.md) is the plan, [ARCHITECTURE.md](ARCHITECTURE.md) will explain how it works as it gets built, [AGENTS.md](AGENTS.md) how to work on it.

## What it will do

**Drive.** A simulated differential-drive robot in a 2D world, driven from the browser with a keyboard or a gamepad, with its pose, speed, battery and joint states streamed back and drawn live, the round-trip latency of every command shown on screen, a command timeout and a dead-man switch in the robot, and an emergency stop that bypasses everything else.

**See.** Low-latency video from the robot's camera over WebRTC, a phone's camera standing in for the robot's at first, with the same view also available over a plain WebSocket so the two paths can be measured against each other.

**Record.** Episodes of synchronized observations and actions written in the LeRobot dataset layout, a flag for marking an episode bad while it is still fresh, and a browser that replays any episode against the recorded video.

**Hand over.** A controller interface with three implementations, the human operator, a scripted routine and a learned policy served over the network, an arbiter that says which one holds each robot, and takeover and handback that leave a trace in the recording.

**Many.** A fleet view with every robot's state, who holds it and its latest frame, and versioned configurations for robots and tasks.

**Real.** A ROS 2 robot through rosbridge, episodes exported as MCAP for Foxglove, and a physical arm behind the same contract.

## Running it

`make run` starts the platform on port 8080 with the simulated robot in-process; `make dev` serves the console on port 3000, which reaches the platform at the address in `.env` (see `.env.example`). Press Enter in the console to engage, drive with the keys or a gamepad, Space for the emergency stop. `go run ./cmd/zebes` runs the simulator as a separate process against a platform started without `-sim`.

To lend a phone's camera to the robot, set `NEXT_PUBLIC_TOURIAN_URL` in `.env` to the laptop's address on the local network, start `make dev`, and open `http://<laptop>:3000/camera?robot=zebes-1` on the phone. Browsers hand out the camera only to secure origins, so on the phone's Chrome add that origin at `chrome://flags/#unsafely-treat-insecure-origin-as-secure` first; the platform is reachable from the laptop alone until authentication arrives, as noted in SCOPE.md. The console shows the picture over WebRTC or MJPEG, chosen above it; `make results-video` measures both paths with a fake camera and writes the figures under `docs/results/`.

## License

MIT; see [LICENSE](LICENSE).
