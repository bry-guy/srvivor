#!/usr/bin/env python3
import pathlib
import re
import sys

if len(sys.argv) != 3:
    print("usage: update-home-k3s-digests.py <image-name> <sha256:digest>", file=sys.stderr)
    sys.exit(2)

image_name = sys.argv[1]
digest = sys.argv[2]
if not digest.startswith("sha256:"):
    print("digest must start with sha256:", file=sys.stderr)
    sys.exit(2)

path = pathlib.Path("deploy/environments/home-k3s/kustomization.yaml")
text = path.read_text()
pattern = re.compile(
    rf"(- name: {re.escape(image_name)}\n\s+newName: {re.escape(image_name)}\n\s+digest: )(sha256:[0-9a-f]{{64}})",
    re.MULTILINE,
)
updated, count = pattern.subn(rf"\1{digest}", text)
if count != 1:
    print(f"failed to update digest for {image_name}", file=sys.stderr)
    sys.exit(1)
path.write_text(updated)
