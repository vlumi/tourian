from tourian.v1 import robot_pb2
from tourian_arm.safety import Guard


def test_guard_holds_stops_and_clears():
    guard = Guard(0.5, 1.0)
    assert guard.halt(0.0) == robot_pb2.HALT_REASON_DEAD_MAN
    guard.heartbeat(0.0)
    assert guard.halt(0.0) == robot_pb2.HALT_REASON_COMMAND_TIMEOUT
    guard.command(0.0)
    assert guard.halt(0.4) == robot_pb2.HALT_REASON_UNSPECIFIED
    assert guard.halt(0.6) == robot_pb2.HALT_REASON_COMMAND_TIMEOUT
    guard.command(0.6)
    assert guard.halt(1.1) == robot_pb2.HALT_REASON_DEAD_MAN
    guard.heartbeat(1.1)
    guard.stop()
    assert guard.halt(1.1) == robot_pb2.HALT_REASON_EMERGENCY_STOP
    guard.command(1.1)
    assert guard.halt(1.1) == robot_pb2.HALT_REASON_UNSPECIFIED
