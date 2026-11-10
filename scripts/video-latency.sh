#!/usr/bin/env sh
# Builds and serves the console with the platform and the simulator, measures video latency on both paths with a fake
# camera, and writes the results as Markdown with the date, commit and machine to the given file.
set -eu

out="$1"
port="${PORT:-3100}"
platform_port="${PLATFORM_PORT:-8090}"
json="$(mktemp)"
trap 'kill $platform $console 2>/dev/null; rm -f "$json"' EXIT

NEXT_PUBLIC_TOURIAN_URL="ws://localhost:$platform_port" npm run build --workspace console >/dev/null
go run ./cmd/tourian -sim -addr ":$platform_port" >/dev/null 2>&1 &
platform=$!
npm run start --workspace console -- --port "$port" >/dev/null 2>&1 &
console=$!
for _ in $(seq 1 50); do
  curl -sf "http://localhost:$port/" >/dev/null 2>&1 && curl -sf "http://localhost:$platform_port/healthz" >/dev/null 2>&1 && break
  sleep 0.2
done

node scripts/video-latency.mjs "http://localhost:$port" zebes-1 "${SCREENSHOT:-}" > "$json"

date="$(date +%Y-%m-%d)"
commit="$(git rev-parse --short HEAD)"
machine="$(sysctl -n machdep.cpu.brand_string 2>/dev/null || uname -m), $(sw_vers -productName 2>/dev/null || uname -s) $(sw_vers -productVersion 2>/dev/null || uname -r)"
field() { node -e 'console.log(JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"))[process.argv[2]])' "$json" "$1"; }

{
  echo "# Video latency, $date"
  echo
  echo "Age of each frame when the console shows it, counted from the moment the camera page composed it, with the camera page and the console in one headless Chrome on the same machine. The age covers encoding, the path, decoding and rendering, not the camera's sensor or the display. Measured by \`make results-video\` with a fake camera at 640×480 for $(field seconds) s per path after a warm-up."
  echo
  echo "- Commit: \`$commit\`"
  echo "- Machine: $machine"
  echo "- Browser: $(field browser)"
  echo "- Platform and camera page on localhost, the simulated robot in-process"
  echo
  echo "| Path | Frames shown | fps | Median | p95 | Min | Max |"
  echo "| --- | --- | --- | --- | --- | --- | --- |"
  node -e '
    const results = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")).results
    const names = { webrtc: "WebRTC, peer to peer", mjpeg: "MJPEG over WebSocket through the platform" }
    for (const [path, s] of Object.entries(results)) {
      console.log(`| ${names[path] ?? path} | ${s.frames} | ${s.fps} | ${s.medianMs} ms | ${s.p95Ms} ms | ${s.minMs} ms | ${s.maxMs} ms |`)
    }
  ' "$json"
} > "$out"
echo "Wrote $out"
