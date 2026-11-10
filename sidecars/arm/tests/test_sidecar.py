"""The sidecar against a stand-in platform: announces, moves on command, stops when heartbeats stop,
stops at once on an emergency stop."""

import asyncio

from google.protobuf.timestamp_pb2 import Timestamp
from websockets.asyncio.server import serve

from tourian.v1 import header_pb2, robot_pb2, transport_pb2
from tourian_arm.driver import FakeArm
from tourian_arm.sidecar import Sidecar


class Platform:
    def __init__(self):
        self.inbox: asyncio.Queue[transport_pb2.Envelope] = asyncio.Queue()
        self.connection = None
        self.connected = asyncio.Event()

    async def handler(self, websocket):
        self.connection = websocket
        self.connected.set()
        async for data in websocket:
            envelope = transport_pb2.Envelope()
            envelope.ParseFromString(data)
            await self.inbox.put(envelope)

    async def send(self, **fields):
        await self.connection.send(transport_pb2.Envelope(**fields).SerializeToString())

    async def next(self, kind: str, timeout: float = 3.0):
        async with asyncio.timeout(timeout):
            while True:
                envelope = await self.inbox.get()
                if envelope.WhichOneof("message") == kind:
                    return getattr(envelope, kind)

    async def until(self, predicate, timeout: float = 3.0) -> robot_pb2.Observation:
        async with asyncio.timeout(timeout):
            while True:
                observation = await self.next("observation")
                if predicate(observation):
                    return observation


def stamp() -> Timestamp:
    now = Timestamp()
    now.GetCurrentTime()
    return now


async def test_sidecar_behaves_like_a_robot_adapter():
    platform = Platform()
    async with serve(platform.handler, "127.0.0.1", 0) as server:
        port = server.sockets[0].getsockname()[1]
        sidecar = Sidecar(f"ws://127.0.0.1:{port}", "arm-1", "Test arm", FakeArm(max_speed=10.0), 0.3, 0.6)
        task = asyncio.create_task(sidecar.run())
        try:
            await asyncio.wait_for(platform.connected.wait(), 3)
            capabilities = await platform.next("capabilities")
            assert capabilities.robot_id == "arm-1"
            assert list(capabilities.joint_names) == ["shoulder", "elbow", "wrist", "gripper"]
            assert capabilities.command_timeout.ToTimedelta().total_seconds() == 0.3

            await platform.until(lambda o: o.halt_reason == robot_pb2.HALT_REASON_DEAD_MAN)

            async def operate(target, *, heartbeat=True, times=8):
                for seq in range(1, times + 1):
                    header = header_pb2.Header(stamp=stamp())
                    if heartbeat:
                        await platform.send(
                            heartbeat=robot_pb2.Heartbeat(header=header, robot_id="arm-1", sequence=seq)
                        )
                    await platform.send(
                        joint_command=robot_pb2.JointCommand(
                            header=header, robot_id="arm-1", sequence=seq, target=robot_pb2.JointState(position=target)
                        )
                    )
                    await asyncio.sleep(0.05)

            await operate([1.0, 0.5, 0.0, 0.0])
            ack = await platform.next("command_ack")
            assert ack.sequence >= 1
            moved = await platform.until(
                lambda o: o.halt_reason == robot_pb2.HALT_REASON_UNSPECIFIED and abs(o.joints.position[0] - 1.0) < 0.01
            )
            assert moved.joints.name[0] == "shoulder"

            held = await platform.until(lambda o: o.halt_reason == robot_pb2.HALT_REASON_DEAD_MAN, timeout=2.0)
            assert abs(held.joints.position[0] - 1.0) < 0.01

            await operate([0.0, 0.0, 0.0, 0.0], times=2)
            await platform.send(stop=robot_pb2.Stop(robot_id="arm-1"))
            stopped = await platform.until(lambda o: o.halt_reason == robot_pb2.HALT_REASON_EMERGENCY_STOP, timeout=1.0)
            resting = stopped.joints.position[0]
            await asyncio.sleep(0.2)
            later = await platform.next("observation")
            assert abs(later.joints.position[0] - resting) < 0.01
        finally:
            task.cancel()
