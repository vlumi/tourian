"""What the sidecar needs from an arm's own driver, and a stand-in arm for running without one."""

import time
from collections.abc import Sequence
from typing import Protocol


class ArmDriver(Protocol):
    """The arm as its driver exposes it: named joints, their positions, a target to move toward,
    and a stop that holds."""

    @property
    def joint_names(self) -> Sequence[str]: ...

    def read(self) -> list[float]: ...

    def move(self, targets: Sequence[float]) -> None: ...

    def stop(self) -> None: ...


class FakeArm:
    """Joints that glide toward their targets at a fixed speed, as a real arm's controller would,
    advanced by the wall clock on every read."""

    def __init__(
        self,
        joint_names: Sequence[str] = ("shoulder", "elbow", "wrist", "gripper"),
        max_speed: float = 1.0,
        clock=time.monotonic,
    ) -> None:
        self._names = tuple(joint_names)
        self._positions = [0.0] * len(self._names)
        self._targets = list(self._positions)
        self._max_speed = max_speed
        self._clock = clock
        self._last = clock()

    @property
    def joint_names(self) -> Sequence[str]:
        return self._names

    def read(self) -> list[float]:
        now = self._clock()
        step = self._max_speed * max(0.0, now - self._last)
        self._last = now
        for i, target in enumerate(self._targets):
            delta = target - self._positions[i]
            self._positions[i] += max(-step, min(step, delta))
        return list(self._positions)

    def move(self, targets: Sequence[float]) -> None:
        self.read()
        self._targets = [float(t) for t in targets][: len(self._names)] + self._targets[len(targets) :]

    def stop(self) -> None:
        self._targets = list(self.read())
