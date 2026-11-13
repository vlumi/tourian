import json
import threading
import urllib.request
from http.server import ThreadingHTTPServer

from google.protobuf.json_format import MessageToJson

from tourian.v1.robot_pb2 import Observation
from tourian_script import _as_twist, twist


def test_a_pair_or_nothing_becomes_a_twist():
    assert _as_twist((0.5, -0.25)).linear.x == 0.5
    assert _as_twist((0.5, -0.25)).angular.z == -0.25
    assert _as_twist(None).linear.x == 0
    assert _as_twist(twist(1, 2)).angular.z == 2


def test_the_server_describes_itself_and_acts(monkeypatch):
    import tourian_script

    captured = {}

    def fake_server(address, handler):
        server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
        captured["server"] = server
        return server

    monkeypatch.setattr(tourian_script, "ThreadingHTTPServer", fake_server)
    thread = threading.Thread(
        target=tourian_script.serve, args=(lambda o: (o.pose.position.x, 0.1),), kwargs={"id": "t", "name": "Test"}
    )
    thread.start()
    while "server" not in captured:
        pass
    server = captured["server"]
    base = f"http://127.0.0.1:{server.server_address[1]}"
    try:
        with urllib.request.urlopen(base + "/") as response:
            assert json.load(response) == {"id": "t", "kind": "CONTROLLER_KIND_SCRIPT", "name": "Test"}
        observation = Observation()
        observation.pose.position.x = 2
        request = urllib.request.Request(base + "/act", data=MessageToJson(observation).encode(), method="POST")
        with urllib.request.urlopen(request) as response:
            assert json.load(response) == {"linear": {"x": 2}, "angular": {"z": 0.1}}
    finally:
        server.shutdown()
        thread.join()
