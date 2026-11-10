"""The sidecar: announces the arm to the platform, moves it on joint commands while the operator is heard,
and holds it otherwise."""

import asyncio
import logging
import time

from google.protobuf.duration_pb2 import Duration
from google.protobuf.timestamp_pb2 import Timestamp
from websockets.asyncio.client import connect
from websockets.exceptions import ConnectionClosed

from tourian.v1 import header_pb2, robot_pb2, transport_pb2

from .driver import ArmDriver
from .safety import Guard

log = logging.getLogger("tourian_arm")

TICK = 0.02
OBSERVE_EVERY = 0.05
RECONNECT_AFTER = 1.0


def _stamp(seconds: float | None = None) -> Timestamp:
    stamp = Timestamp()
    stamp.FromNanoseconds(int((time.time() if seconds is None else seconds) * 1e9))
    return stamp


def _duration(seconds: float) -> Duration:
    duration = Duration()
    duration.FromNanoseconds(int(seconds * 1e9))
    return duration


class Sidecar:
    def __init__(
        self,
        platform_url: str,
        robot_id: str,
        name: str,
        driver: ArmDriver,
        command_timeout: float = 0.5,
        heartbeat_timeout: float = 1.0,
        clock=time.monotonic,
    ) -> None:
        self.platform_url = platform_url
        self.robot_id = robot_id
        self.name = name
        self.driver = driver
        self.command_timeout = command_timeout
        self.heartbeat_timeout = heartbeat_timeout
        self.clock = clock
        self.guard = Guard(command_timeout, heartbeat_timeout)
        self.target: list[float] | None = None
        self.last_sequence = 0
        self.halt = robot_pb2.HALT_REASON_DEAD_MAN

    def capabilities(self) -> robot_pb2.Capabilities:
        return robot_pb2.Capabilities(
            robot_id=self.robot_id,
            name=self.name,
            joint_names=list(self.driver.joint_names),
            command_timeout=_duration(self.command_timeout),
            heartbeat_timeout=_duration(self.heartbeat_timeout),
        )

    def observation(self) -> robot_pb2.Observation:
        positions = self.driver.read()
        return robot_pb2.Observation(
            header=header_pb2.Header(stamp=_stamp(), frame_id="base_link"),
            robot_id=self.robot_id,
            joints=robot_pb2.JointState(name=list(self.driver.joint_names), position=positions),
            last_command_sequence=self.last_sequence,
            halt_reason=self.halt,
        )

    def handle(self, envelope: transport_pb2.Envelope) -> robot_pb2.CommandAck | None:
        """Applies one message from the platform; a joint command is acknowledged at once."""
        now = self.clock()
        kind = envelope.WhichOneof("message")
        if kind == "joint_command":
            command = envelope.joint_command
            self.target = list(command.target.position)
            self.last_sequence = command.sequence
            self.guard.command(now)
            return robot_pb2.CommandAck(
                header=header_pb2.Header(stamp=_stamp()),
                robot_id=self.robot_id,
                sequence=command.sequence,
                command_stamp=command.header.stamp,
            )
        if kind == "heartbeat":
            self.guard.heartbeat(now)
        elif kind == "stop":
            self.guard.stop()
            self.target = None
            self.driver.stop()
        return None

    def tick(self) -> None:
        """Moves toward the target while the operator is heard, holds the arm the moment not."""
        self.halt = self.guard.halt(self.clock())
        if self.halt != robot_pb2.HALT_REASON_UNSPECIFIED:
            self.driver.stop()
        elif self.target is not None:
            self.driver.move(self.target)

    async def run(self) -> None:
        """Keeps the arm connected to the platform until cancelled, reconnecting when the platform goes away."""
        while True:
            try:
                await self.session()
            except (OSError, ConnectionClosed) as error:
                log.warning("connection to %s ended: %s; retrying in %ss", self.platform_url, error, RECONNECT_AFTER)
            await asyncio.sleep(RECONNECT_AFTER)

    async def session(self) -> None:
        async with connect(self.platform_url, max_size=1 << 20) as ws:
            await ws.send(transport_pb2.Envelope(capabilities=self.capabilities()).SerializeToString())
            log.info("%s connected to %s", self.robot_id, self.platform_url)

            async def read() -> None:
                async for data in ws:
                    envelope = transport_pb2.Envelope()
                    envelope.ParseFromString(data)
                    if ack := self.handle(envelope):
                        await ws.send(transport_pb2.Envelope(command_ack=ack).SerializeToString())

            async def drive() -> None:
                last_observed = 0.0
                while True:
                    self.tick()
                    now = self.clock()
                    if now - last_observed >= OBSERVE_EVERY:
                        last_observed = now
                        await ws.send(transport_pb2.Envelope(observation=self.observation()).SerializeToString())
                    await asyncio.sleep(TICK)

            async with asyncio.TaskGroup() as tasks:
                tasks.create_task(read())
                tasks.create_task(drive())
