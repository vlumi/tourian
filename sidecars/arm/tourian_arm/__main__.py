"""Run the sidecar from the command line, with the stand-in arm until a real driver is wired in."""

import argparse
import asyncio
import logging

from .driver import FakeArm
from .sidecar import Sidecar


def main() -> None:
    parser = argparse.ArgumentParser(description="A physical arm behind the tourian contract")
    parser.add_argument("--platform", default="ws://localhost:8080/robot", help="the platform's robot endpoint")
    parser.add_argument("--id", default="arm-1", help="robot id")
    parser.add_argument("--name", default="Arm", help="robot name")
    parser.add_argument("--command-timeout", type=float, default=0.5)
    parser.add_argument("--heartbeat-timeout", type=float, default=1.0)
    args = parser.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s: %(message)s")
    sidecar = Sidecar(args.platform, args.id, args.name, FakeArm(), args.command_timeout, args.heartbeat_timeout)
    try:
        asyncio.run(sidecar.run())
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
