#!/usr/bin/env sh
# Installs the npm packages and the Python environment, with the generated Python contract on its path.
set -eu

npm ci
python3 -m venv .venv
.venv/bin/pip install -q -e 'sidecars/arm[dev]' -e 'sidecars/script[dev]'
.venv/bin/python - <<'PY'
import pathlib, site
pathlib.Path(site.getsitepackages()[0], "tourian-gen.pth").write_text(str(pathlib.Path("gen/python").resolve()) + "\n")
PY
