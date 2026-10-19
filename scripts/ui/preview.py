#!/usr/bin/env python3
"""Serve only the gallery and its public assets, never the repository root."""
import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from tempfile import TemporaryDirectory
import shutil

parser = argparse.ArgumentParser()
parser.add_argument("--port", type=int, default=18883)
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
assets = ["scripts/ui/design-system.html", "internal/web/assets/app.css",
          "internal/web/assets/ui.js", "internal/modules/built_in_relay/assets/style.css",
          "internal/modules/ports_over_relay/assets/style.css"]
with TemporaryDirectory(prefix="zelport-ui-") as directory:
    for name in assets:
        target = Path(directory) / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(root / name, target)
    server = ThreadingHTTPServer(("127.0.0.1", args.port),
                                partial(SimpleHTTPRequestHandler, directory=directory))
    print(f"Gallery: http://127.0.0.1:{args.port}/scripts/ui/design-system.html", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
