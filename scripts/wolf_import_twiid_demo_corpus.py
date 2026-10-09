#!/usr/bin/env python3
"""Import The Way ID Did's native demos and matching maps into a local corpus."""

import argparse
import hashlib
import io
import json
from pathlib import Path
import sys
import zipfile


ARCHIVE_SHA256 = "5f00178093f396b8513c6835ac17c8f7384b4aecccd11e210e7e00c3e7afeba7"
SOURCE_URL = "https://www.moddb.com/downloads/wolfenstein-3d-the-way-id-did"
MIRROR_PAGE = "https://beta.wolf3d.net/game/2462"
DOWNLOAD_URL = "https://beta.wolf3d.net/file-download/download/private/2406"
BASE_NAMES = ("VSWAP.WL6", "AUDIOHED.WL6", "AUDIOT.WL6")
MOD_NAMES = ("MAPHEAD.WL6", "GAMEMAPS.WL6", "VGADICT.WL6", "VGAHEAD.WL6", "VGAGRAPH.WL6")


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive", type=Path, required=True, help="author's July 3, 2021 ZIP (download separately)")
    parser.add_argument("--registered-data", type=Path, required=True, help="local registered WL6 assets")
    parser.add_argument("--out", type=Path, required=True, help="new isolated corpus directory, preferably under build/")
    args = parser.parse_args()
    archive = args.archive.read_bytes()
    if digest(archive) != ARCHIVE_SHA256:
        raise ValueError("archive SHA-256 differs from the verified July 3, 2021 release")
    if args.out.exists():
        raise ValueError(f"output directory already exists: {args.out}")
    base = args.registered_data.resolve()
    data = {name: (base / name).read_bytes() for name in BASE_NAMES}
    with zipfile.ZipFile(io.BytesIO(archive)) as package:
        data.update({name: package.read(name) for name in MOD_NAMES})
        with zipfile.ZipFile(io.BytesIO(package.read("W3DTWID.pk3"))) as pk3:
            demos = {f"DEMO{i}.WL6": pk3.read(f"DEMO{i}.WL6") for i in range(4)}
    records = []
    for index, (name, raw) in enumerate(demos.items()):
        if len(raw) < 7 or int.from_bytes(raw[1:3], "little") != len(raw) or (len(raw) - 4) % 3 or raw[0] >= 60:
            raise ValueError(f"invalid original-format recording: {name}")
        records.append({
            "id": f"twiid-{index}", "file": f"recordings/{name}", "data": "data",
            "source_url": SOURCE_URL,
            "recorded_version": "The Way ID Did, July 3 2021 archive; native W3DTWID.pk3 recordings; recorder executable version unspecified",
            "sha256": digest(raw), "map": raw[0], "commands": (len(raw) - 4) // 3,
            "data_sha256": {name: digest(raw) for name, raw in data.items()},
        })
    out = args.out.resolve()
    (out / "data").mkdir(parents=True)
    (out / "recordings").mkdir()
    # Read only known members, never archive paths or bundled executables.
    for name, raw in data.items():
        (out / "data" / name).write_bytes(raw)
    for name, raw in demos.items():
        (out / "recordings" / name).write_bytes(raw)
    provenance = {
        "source_url": SOURCE_URL, "mirror_page": MIRROR_PAGE, "download_url": DOWNLOAD_URL,
        "archive_sha256": ARCHIVE_SHA256, "archive_md5": hashlib.md5(archive).hexdigest(),
        "registered_base": str(base), "mod_data_files": list(MOD_NAMES), "base_data_files": list(BASE_NAMES),
        "data_files": {name: digest(raw) for name, raw in data.items()},
    }
    (out / "provenance.json").write_text(json.dumps(provenance, indent=2) + "\n")
    (out / "corpus.json").write_text(json.dumps({"version": 1, "recordings": records}, indent=2) + "\n")
    print(f"Imported four native recordings with matching maps: {out / 'corpus.json'}")
    print("Run with scripts/wolf_demo_corpus_compare.py --external-only --manifest " + str(out / "corpus.json"))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, zipfile.BadZipFile) as error:
        print(f"wolf_import_twiid_demo_corpus: {error}", file=sys.stderr)
        sys.exit(2)
