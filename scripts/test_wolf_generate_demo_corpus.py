import hashlib
import json
from pathlib import Path
import struct
import tempfile
import unittest

import wolf_generate_demo_corpus as corpus


class GeneratedDemoCorpusTest(unittest.TestCase):
    def test_native_format_boundaries(self):
        raw = corpus.native_demo(59, [(255, -128, 127), (0, 0, -1)])
        self.assertEqual(raw, bytes([59, 10, 0, 0, 255, 128, 127, 0, 0, 255]))
        longest = corpus.native_demo(0, [(0, 0, 0)] * corpus.MAX_COMMANDS)
        self.assertEqual(len(longest), struct.unpack_from("<H", longest, 1)[0])
        for index, commands in [(60, [(0, 0, 0)]), (0, []), (0, [(0, 0, 0)] * (corpus.MAX_COMMANDS + 1))]:
            with self.assertRaises(ValueError):
                corpus.native_demo(index, commands)

    def test_reproducible_varied_controls(self):
        idle = corpus.route_commands("idle", 512, 123)
        self.assertEqual(set(idle), {(0, 0, 0)})
        combat = corpus.route_commands("combat", 512, 123)
        self.assertEqual(combat, corpus.route_commands("combat", 512, 123))
        self.assertEqual(hashlib.sha256(corpus.native_demo(0, combat)).hexdigest(),
                         "403051a7014a4edf15f9b5b4ace229bab6b0478164f82b74c952a337d3547106")
        self.assertNotEqual(combat, corpus.route_commands("combat", 512, 124))
        self.assertEqual(len(combat), 512)
        self.assertEqual(combat[:8], idle[:8])
        self.assertTrue(any(buttons & corpus.ATTACK for buttons, _, _ in combat))
        self.assertTrue(any(buttons & corpus.STRAFE for buttons, _, _ in combat))
        self.assertTrue(any(x < 0 for _, x, _ in combat))
        self.assertTrue(any(y < 0 for _, _, y in combat))
        self.assertTrue(any(y > 0 for _, _, y in combat))
        self.assertFalse(any(buttons & corpus.ATTACK for buttons, _, _ in corpus.route_commands("patrol", 512, 123)))

    def test_map_selection(self):
        self.assertEqual(corpus.map_selection("0-2,48,59"), [0, 1, 2, 48, 59])
        for value in ("", "0,0", "3-1", "60", "-1", "0-2-4"):
            with self.assertRaises(ValueError):
                corpus.map_selection(value)

    def test_all_maps_manifest_and_no_overwrite(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            data = root / "data"
            data.mkdir()
            for name in corpus.DATA_NAMES:
                (data / (name + ".WL6")).write_bytes(name.encode())
            out = root / "generated"
            manifest = corpus.generate(data, out, list(range(60)), ["idle", "combat"], [0, 7], 32)
            self.assertEqual(len(manifest["recordings"]), 240)
            self.assertEqual(json.loads((out / "corpus.json").read_text()), manifest)
            for record in manifest["recordings"]:
                raw = (out / record["file"]).read_bytes()
                self.assertEqual(raw[0], record["map"])
                self.assertEqual(len(raw), 100)
                self.assertEqual(hashlib.sha256(raw).hexdigest(), record["sha256"])
                self.assertEqual(len(record["data_sha256"]), 8)
            with self.assertRaises(ValueError):
                corpus.generate(data, out, [0], ["idle"], [0], 1)


if __name__ == "__main__":
    unittest.main()
