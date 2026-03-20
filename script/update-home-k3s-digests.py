#!/usr/bin/env python3
import pathlib
import re
import sys

KUSTOMIZATION_PATH = pathlib.Path("deploy/environments/home-k3s/kustomization.yaml")
IMAGE_PATTERN_TEMPLATE = r"(- name: {image}\n\s+newName: {image}\n\s+digest: )(sha256:[0-9a-f]{{64}})"

if len(sys.argv) != 3:
    print("usage: update-home-k3s-digests.py <image-name> <sha256:digest>", file=sys.stderr)
    sys.exit(2)

image_name = sys.argv[1]
digest = sys.argv[2]
if not digest.startswith("sha256:"):
    print("digest must start with sha256:", file=sys.stderr)
    sys.exit(2)

if not KUSTOMIZATION_PATH.exists():
    print(f"expected active selfhost overlay at {KUSTOMIZATION_PATH}", file=sys.stderr)
    sys.exit(1)

text = KUSTOMIZATION_PATH.read_text()
pattern = re.compile(
    IMAGE_PATTERN_TEMPLATE.format(image=re.escape(image_name)),
    re.MULTILINE,
)
updated, count = pattern.subn(rf"\1{digest}", text)
if count != 1:
    print(
        f"failed to update digest for {image_name} in {KUSTOMIZATION_PATH}",
        file=sys.stderr,
    )
    sys.exit(1)

KUSTOMIZATION_PATH.write_text(updated)
