# tourian

A control room for robots, and for whatever is at the controls.

Tourian is a teleoperation platform: an operator drives a robot from a browser over a live video feed, every observation and action is recorded as an episode for training, and control of any robot can be handed from the human to a scripted routine or a learned policy and taken back, with a kill switch that works even when nothing else does. The robots start simulated; the seams are placed so that a ROS 2 robot or a physical arm slots in behind the same contract.

It is an experiment for understanding the domain by building it, not a product. The same console drives every robot through one Protobuf contract, with the transport, the video path, the dataset format and the robot itself each swappable, so that technologies can be compared against each other on the same system.

**The name:** Tourian is the sector of planet Zebes where Mother Brain sits. The brain is the controller, whoever holds it; the sector is the platform. The components keep the theme: `motherbrain` for the controller that holds a robot, `emmi` for the robot adapters, `zebes` for the simulated world, `samus` for the operator. Named after locations and characters in Nintendo's Metroid series; not affiliated with or endorsed by Nintendo.

**Status:** bootstrapped 2026-11-10; the simulated robot drives from the console over WebSocket with the safety behaviors in the robot (M1), a phone's camera shows on the console over WebRTC or MJPEG, both paths measured (M2), episodes are recorded as JSONL and in the LeRobot layout, flagged and replayed (M3), control passes between the operator, a scripted square and a policy server, every change stamped into the episode (M4), several robots run under one platform with a fleet view and versioned configurations of robots and tasks (M5), and a ROS 2 robot through rosbridge, episodes as MCAP and a Python sidecar for an arm sit behind the same contract (M6, short of a hardware test). [SCOPE.md](SCOPE.md) is the plan, [ARCHITECTURE.md](ARCHITECTURE.md) explains how it works, [AGENTS.md](AGENTS.md) how to work on it.

## What it does

**Drive.** A simulated differential-drive robot in a 2D world, driven from the browser with a keyboard or a gamepad, with its pose, speed, battery and joint states streamed back and drawn live, the round-trip latency of every command shown on screen, a command timeout and a dead-man switch in the robot, and an emergency stop that bypasses everything else.

**See.** Low-latency video from the robot's camera over WebRTC, a phone's camera standing in for the robot's at first, with the same view also available over a plain WebSocket so the two paths can be measured against each other.

**Record.** Episodes of synchronized observations and actions written in the LeRobot dataset layout, a flag for marking an episode bad while it is still fresh, and a browser that replays any episode against the recorded video.

**Hand over.** A controller interface with three implementations, the human operator, a scripted routine and a learned policy served over the network, an arbiter that says which one holds each robot, and takeover and handback that leave a trace in the recording.

**Many.** A fleet view with every robot's state, who holds it and its latest frame, and versioned configurations for robots and tasks.

**Real.** A ROS 2 robot through rosbridge, episodes exported as MCAP for Foxglove, and a physical arm behind the same contract.

## Running it

`make tools` and `make deps` once, then `make run` starts the platform on port 8080 with the simulated robot in-process and `make dev` serves the console on port 3000. The console reaches the platform at `NEXT_PUBLIC_TOURIAN_URL`, read from `.env` at the root (see `.env.example`) when `make dev` or `make build` starts, since the value is inlined then; change it and restart. Press Enter in the console to engage, drive with W, A, S, D or the arrows, or a gamepad (left stick drives, right stick turns, Start engages, B stops), Space for the emergency stop; losing the window's focus releases. `go run ./cmd/zebes` runs the simulator as a separate process against a platform started without `-sim`. The platform's flags: `-addr`, `-data` for where recordings go, `-lerobot` and `-mcap` for the dataset layouts, `-policy` for a policy server, `-sim` and `-sims`.

To lend a phone's camera to the robot, set `NEXT_PUBLIC_TOURIAN_URL` in `.env` to the laptop's address on the local network, start `make dev`, and open `http://<laptop>:3000/camera?robot=zebes-1` on the phone. Browsers hand out the camera only to secure origins, so on the phone's Chrome add that origin at `chrome://flags/#unsafely-treat-insecure-origin-as-secure` first. The platform listens on every interface and has no authentication, so it must stay on a network you trust until that arrives, as noted in SCOPE.md. The console shows the picture over WebRTC or MJPEG, chosen above it; `make results-video` measures both paths with a fake camera and writes the figures under `docs/results/`.

Several robots: `make run` runs one simulated robot; `go run ./cmd/tourian -sim -sims 3` runs three, or `go run ./cmd/zebes -id zebes-2 -y 2` adds one from another process. With more than one, a fleet strip above the picture shows each robot's holder, state and latest picture, and clicking one selects it for driving. The console's Config page sets up robots (a name, speed limits under the robot's own, parameters) and tasks (name, description, what success looks like); every save is a new version, kept under `data/config/`, and an episode records the versions it was made on and for.

Other robots: `go run ./cmd/rossim` runs a simulated ROS 2 robot behind the rosbridge protocol on port 9090, and `go run ./cmd/rosbridge -joints wheel_left,wheel_right` bridges it, or a real `rosbridge_server`, to the platform. `make deps` sets up `.venv`, and `cd sidecars/arm && PYTHONPATH=../../gen/python ../../.venv/bin/python -m tourian_arm` runs the arm sidecar with its stand-in arm. Every episode is also written as MCAP under `data/mcap/` for Foxglove.

Control: the console's panel says who holds the robot and hands it to the scripted square or, when the platform was started with `-policy http://localhost:8091`, to a policy server; `go run ./cmd/policy` is a stand-in that steers home to the origin over the same protocol a learned policy's server would speak. Engaging takes the robot over from whoever holds it, releasing hands it back.

Recording writes under `data/`: each episode as a directory of JSONL and JPEG files under `data/episodes/`, the same frames into one LeRobot v3.0 dataset per robot under `data/lerobot/tourian/<robot id>/`, and an MCAP file under `data/mcap/`. A dataset's layout is fixed by its first episode: it gets a camera feature only if that episode had pictures and `ffmpeg` was on the path when the platform started; otherwise the pictures stay in the JSONL layout. The console's Episodes page, at `/episodes`, lists and replays what was recorded; `/config` edits the configurations.

## License

MIT; see [LICENSE](LICENSE).
