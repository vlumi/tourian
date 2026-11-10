# tourian — scope

A teleoperation platform for robots, simulated first, with recording for training data and a handover of control between a human, a script and a learned policy. A sibling of [fiftysixty](https://github.com/vlumi/fiftysixty) in spirit: built to understand a domain by building it, and like [nebulosa](https://github.com/vlumi/nebulosa) on a toolchain that is copied rather than reinvented.

**The name:** Tourian is where Mother Brain sits. The platform is the sector; the brain is whoever is at the controls. See the README for the component names.

**Status:** experiment, not a product. Bootstrapped 2026-11-10; no milestone done yet. Milestones get their dates on the headings as they land.

## The idea

Teleoperation is three loops running at once: a control loop from the operator's hands to the robot's actuators, a perception loop from the robot's camera to the operator's eyes, and a recording loop that writes both down as training data. A learned policy is just another thing that can sit in the control loop. Tourian builds the three loops once, behind one contract, and makes the parts on either side of the contract swappable: which robot, over which transport, with which video path, into which dataset format, under whose control.

## The seams

Five interfaces, each with a first implementation and a planned second one, so that none of them stays a one-off abstraction.

| Seam | What crosses it | First | Then |
| --- | --- | --- | --- |
| Robot (`emmi`) | commands in, observations and capabilities out | the 2D simulator | ROS 2 via rosbridge; a physical arm through a Python sidecar |
| Transport | contract messages between console, platform and robot | WebSocket | WebRTC data channel; MQTT or gRPC streams for comparison |
| Video | frames or a stream from the robot's camera | WebRTC from a phone's camera | MJPEG over WebSocket for the comparison; a ROS 2 image topic; a simulated camera |
| Dataset sink | episodes of observations and actions | JSONL for debugging | the LeRobot layout; MCAP for Foxglove |
| Controller (`motherbrain`) | the thing that holds a robot and issues its commands | the human operator | a scripted routine; a learned policy behind a policy server |

The contract is Protobuf in `proto/`, with Go and TypeScript generated and committed. Messages follow ROS 2 conventions where one exists, so a velocity command is a Twist, joints are a JointState and every observation carries a frame id and a timestamp; that keeps the ROS 2 adapter a translation rather than a redesign.

Safety is part of the robot seam, not the console: every adapter stops its robot when commands stop arriving within the timeout, honors a dead-man signal, and executes an emergency stop ahead of anything in its queue. The simulator implements all three so the behavior is tested long before anything physical is attached.

## Tech

- **Platform and robots:** Go, one module, `cmd/` for the binaries, packages by feature. Pion for WebRTC. The simulator is a differential-drive robot in a 2D world with a few fake joints, a battery that drains and a camera pose.
- **Console:** Next.js with TypeScript and React, the generated contract types, a canvas for the world and the robot, a gamepad through the Gamepad API, a video element for the WebRTC track.
- **Contract:** Protobuf with `buf`, generating Go and TypeScript; messages for commands, observations, capabilities, episodes, control arbitration and configuration.
- **Datasets:** the LeRobot layout (Parquet tables plus video files plus metadata JSON) as the primary format, written from Go; JSONL as the debug sink; MCAP export for Foxglove.
- **Policy server:** a learned policy runs out of process and is reached over the network; the controller adapter speaks whatever that server speaks, so a public policy-serving protocol can be used without binding the platform to one framework.
- **Tooling:** `make` for everything routine, `make ci` as the gate; hosted CI runs the same.
- **License:** MIT.

## Milestones

### M0 — bootstrap

Toolchain and layout: Go module, `buf` with a first contract (velocity command, pose and joint observations, capabilities, heartbeat and stop), generated code committed, a Next.js console that renders a page under the name, `make ci` with formatting, linters, generation check and unit tests, hosted CI running the same, a screenshot script.

### M1 — drive

- The 2D simulator as the first robot adapter: differential-drive kinematics, pose, speed, battery, a couple of joints, capabilities announced on connect.
- WebSocket as the first transport, carrying contract messages both ways.
- The console: the world and the robot on a canvas, keyboard and gamepad driving, a telemetry panel with sparklines, round-trip latency of each command from stamped echoes.
- Safety in the simulator: command timeout stops the robot, a dead-man signal from the console, an emergency stop that preempts the command queue. Tests for all three.

### M2 — see

- WebRTC video from a phone's camera to the console, the Go platform signaling; a page the phone opens that publishes its camera as the robot's.
- The same frames over MJPEG on the WebSocket as the second video implementation, and a measurement of glass-to-glass latency on both paths, recorded under `docs/results/` with date, commit and machine.

### M3 — record

- Episodes: synchronized observations and actions, with the video, written through the dataset sink; JSONL first, then the LeRobot layout.
- A record button, an episode-level flag for marking an episode bad while it is fresh, and a browser that lists episodes and replays one against its video.

### M4 — hand over

- The controller interface, extracted now that the human operator has company: a scripted routine that drives a pattern, and a learned policy behind a policy server over the network.
- An arbiter per robot that says who holds it; takeover by the operator at any time, handback when they let go; every change of hands stamped into the episode.
- The console shows who holds each robot and lets the operator switch.

### M5 — many

- Several robots under one platform: a fleet view with each robot's state, holder and latest frame, and one selected for driving.
- Versioned configurations for robots and tasks, edited from the console, stored with the episodes so a recording says what it was recorded on and for.

### M6 — real

- A ROS 2 robot through rosbridge as the second robot adapter, against a simulated ROS 2 robot first.
- Episodes exported as MCAP and opened in Foxglove.
- A physical arm behind the contract through a Python sidecar that uses the arm's own driver; safety behavior retested on hardware before any driving.

### Later

- MQTT and gRPC streams as further transports, for comparison.
- A 3D simulated world with a simulated camera.
- A second console, native, against the same contract.

## Known limits

- The simulator is kinematic; nothing about contact, friction or weight.
- Video is one camera per robot until M5 proves the need for more.
- No authentication until the platform is reachable from anywhere but a laptop; when it is, it comes before anything else.
