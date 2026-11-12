# tourian

A control room for robots, and for whatever is at the controls.

An operator drives a robot from a browser over live video. Every observation and action is recorded as an episode for training. Control can pass from the human to a script or a learned policy and back, with a kill switch that works even when nothing else does. The robots start simulated; a ROS 2 robot or a physical arm slots in behind the same contract.

It is an experiment for learning the domain by building it, not a product. One Protobuf contract runs through every seam, so the transport, the video path, the dataset format and the robot itself can each be swapped and compared on the same system.

[ARCHITECTURE.md](ARCHITECTURE.md) explains how it works, [SCOPE.md](SCOPE.md) what was planned and what is still missing, [AGENTS.md](AGENTS.md) how to work on it.

**The name.** Tourian is the sector of planet Zebes where Mother Brain sits: the brain is whoever holds the controls, the sector is the platform. The parts keep the theme: `motherbrain` for the controllers, `emmi` for the robot adapters, `zebes` for the simulated world, `samus` for the operator. Named after places and characters in Nintendo's Metroid series; not affiliated with or endorsed by Nintendo.

## What it does

- **Drive.** A simulated robot in a 2D world, from a keyboard or a gamepad, with its state streamed back and drawn live, the round trip of every command on screen, and a command timeout, a dead-man switch and an emergency stop in the robot.
- **See.** The robot's camera over WebRTC or over MJPEG through the platform, a phone standing in for the camera, and both paths measured.
- **Record.** Episodes of synchronized observations and actions as JSONL, in the LeRobot layout and as MCAP; a flag for a bad episode; replay against the recorded pictures.
- **Hand over.** The operator, a scripted routine or a policy server at the controls, an arbiter that says who holds each robot, and every change of hands in the recording.
- **Many.** Several robots under one platform, a fleet view, and versioned configurations of robots and tasks.
- **Real.** A ROS 2 robot through rosbridge and a Python sidecar for an arm behind the same contract, short of a hardware test.

## Running it

Setup, once:

1. `make tools` installs buf, protoc-gen-go and golangci-lint into `bin/`.
2. `make deps` installs the npm packages and the Python environment.
3. `cp .env.example .env`; nothing in it needs changing on one machine.

Drive the simulator:

1. `make run` starts the platform on port 8080 with a simulated robot in-process, and on 8443 over https with a certificate of its own.
2. `make dev` serves the console on port 1499; open it there, or through the platform at `https://localhost:8443`.
3. Enter engages, W, A, S, D or the arrows drive, Space is the emergency stop. On a gamepad the left stick drives, the right stick turns, Start engages, B stops. Losing the window's focus releases.

Lend a phone's camera:

1. The console shows a QR code where the picture will be; it opens the camera page on the platform's https address on the local network.
2. The first time on a phone, open the certificate link beside the code, install it, and on iOS turn it on under Settings › General › About › Certificate Trust Settings. Browsers hand out the camera only to secure origins, and the platform's certificate is its own.
3. Scan the code. The console shows the picture over WebRTC or MJPEG, chosen above it.

Hand over control:

- The control panel says who holds the robot and hands it to the scripted square. Engaging takes the robot over from whoever holds it; releasing hands it back; one operator at a time.
- `go run ./cmd/policy` is a stand-in policy server that steers home; start the platform with `-policy http://localhost:8091` to offer it.

Record:

- The Record button records the selected robot; the task comes from the configured tasks; a flag marks an episode bad.
- Episodes land under `data/`: JSONL and JPEG files under `data/episodes/`, one LeRobot v3.0 dataset per robot under `data/lerobot/tourian/`, and an MCAP file per episode under `data/mcap/`. A LeRobot dataset gets its video only if `ffmpeg` is on the path when the platform starts.
- The console's Episodes page lists and replays them; the Config page edits robots and tasks, every save a new version under `data/config/`.

More robots:

- `go run ./cmd/tourian -sim -sims 3` runs three simulated robots; `go run ./cmd/zebes -id zebes-2 -y 2` adds one from another process. A fleet strip appears with more than one.
- `go run ./cmd/rossim` runs a simulated ROS 2 robot behind the rosbridge protocol on port 9090, and `go run ./cmd/rosbridge -joints wheel_left,wheel_right` bridges it, or a real `rosbridge_server`, to the platform.
- `cd sidecars/arm && PYTHONPATH=../../gen/python ../../.venv/bin/python -m tourian_arm` runs the arm sidecar with its stand-in arm.

Platform flags: `-addr` and `-https` for where it listens, `-console` for the console server it serves behind itself, `-lan` for the address phones use, `-data` for where recordings go, `-lerobot` and `-mcap` for the dataset layouts, `-policy` for a policy server, `-sim` and `-sims`, `-origins` for browser origins to let in besides the platform's own host and localhost.

The platform listens on every interface and has no authentication, so it stays on a network you trust until that arrives. `make ci` is the gate; `make` lists the rest, including `make screenshot` and `make results-video`.

## License

MIT; see [LICENSE](LICENSE).
