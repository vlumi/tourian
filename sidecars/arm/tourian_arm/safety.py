"""The part of every robot adapter that must work with the console gone."""

from tourian.v1 import robot_pb2


class Guard:
    """Holds the arm until the first heartbeat and command, stops it when either goes quiet,
    and keeps an emergency stop until a new command arrives."""

    def __init__(self, command_timeout: float, heartbeat_timeout: float) -> None:
        self.command_timeout = command_timeout
        self.heartbeat_timeout = heartbeat_timeout
        self.last_command: float | None = None
        self.last_heartbeat: float | None = None
        self.stopped = False

    def command(self, now: float) -> None:
        self.last_command = now
        self.stopped = False

    def heartbeat(self, now: float) -> None:
        self.last_heartbeat = now

    def stop(self) -> None:
        self.stopped = True

    def halt(self, now: float) -> int:
        if self.stopped:
            return robot_pb2.HALT_REASON_EMERGENCY_STOP
        if self.last_heartbeat is None or now - self.last_heartbeat > self.heartbeat_timeout:
            return robot_pb2.HALT_REASON_DEAD_MAN
        if self.last_command is None or now - self.last_command > self.command_timeout:
            return robot_pb2.HALT_REASON_COMMAND_TIMEOUT
        return robot_pb2.HALT_REASON_UNSPECIFIED
