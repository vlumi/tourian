"""A controller for tourian as one function: it is given each observation and answers with a twist.

The platform asks over HTTP, the way it asks a policy server, so a script runs as its own process in any language;
this is the Python way. Start the platform with -policy pointing at the script's port, and it is offered on the
console like the built-in square.
"""

import logging
from collections.abc import Callable
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from google.protobuf.json_format import MessageToJson, Parse

from tourian.v1.control_pb2 import CONTROLLER_KIND_SCRIPT, Controller
from tourian.v1.geometry_pb2 import Twist
from tourian.v1.robot_pb2 import Observation

Act = Callable[[Observation], Twist | tuple[float, float] | None]

log = logging.getLogger("tourian_script")


def twist(linear: float = 0.0, angular: float = 0.0) -> Twist:
    """A twist driving forward at linear m/s and turning at angular rad/s."""
    t = Twist()
    t.linear.x = linear
    t.angular.z = angular
    return t


def serve(act: Act, *, id: str, name: str, port: int = 8091) -> None:
    """Serves act as a controller until interrupted. act may return a Twist, a (linear, angular) pair, or None to stop."""
    description = Controller(id=id, kind=CONTROLLER_KIND_SCRIPT, name=name)

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            if self.path != "/":
                self.send_error(404)
                return
            self._json(MessageToJson(description))

        def do_POST(self) -> None:
            if self.path != "/act":
                self.send_error(404)
                return
            length = int(self.headers.get("Content-Length", "0"))
            observation = Parse(self.rfile.read(length), Observation())
            try:
                answer = act(observation)
            except Exception:
                log.exception("%s failed on an observation", id)
                self.send_error(500)
                return
            self._json(MessageToJson(_as_twist(answer)))

        def _json(self, body: str) -> None:
            data = body.encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def log_message(self, format: str, *args: object) -> None:
            pass

    server = ThreadingHTTPServer(("", port), Handler)
    log.info("%s serving on :%d; start the platform with -policy http://localhost:%d", name, port, port)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


def _as_twist(answer: Twist | tuple[float, float] | None) -> Twist:
    if answer is None:
        return Twist()
    if isinstance(answer, Twist):
        return answer
    linear, angular = answer
    return twist(linear, angular)
