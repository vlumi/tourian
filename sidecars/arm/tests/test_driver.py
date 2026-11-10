from tourian_arm.driver import FakeArm


class Clock:
    def __init__(self):
        self.now = 0.0

    def __call__(self):
        return self.now


def test_fake_arm_glides_and_holds():
    clock = Clock()
    arm = FakeArm(("a", "b"), max_speed=1.0, clock=clock)
    arm.move([1.0, -2.0])
    clock.now = 0.5
    assert arm.read() == [0.5, -0.5]
    arm.stop()
    clock.now = 2.0
    assert arm.read() == [0.5, -0.5]
    arm.move([1.0])
    clock.now = 3.0
    assert arm.read() == [1.0, -0.5]
