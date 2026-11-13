"""Drives in a circle, faster the lower the battery is not: a script with eyes on the observation."""

import logging

from tourian_script import serve, twist

logging.basicConfig(level=logging.INFO)


def act(observation):
    battery = observation.battery.percentage if observation.HasField("battery") else 1.0
    return twist(linear=0.3 * battery, angular=0.5)


serve(act, id="circle", name="Circle")
