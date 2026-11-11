#!/usr/bin/env sh
# Builds and serves the console with the platform and the simulator, measures video latency on both paths with a fake
# camera, and writes the results as Markdown with the date, commit and machine under docs/results/, or to OUT.
set -eu

date="$(date +%Y-%m-%d)"
out="${OUT:-docs/results/$date-video-latency.md}"
mkdir -p "$(dirname "$out")"
json="$(mktemp)"
trap 'kill $platform $console 2>/dev/null; rm -rf "$build" "$json"' EXIT
. scripts/serve.sh

node scripts/video-latency.mjs "http://localhost:$port" zebes-1 "${SCREENSHOT:-}" > "$json"

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
