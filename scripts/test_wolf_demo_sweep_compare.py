import json
from pathlib import Path
import tempfile
import unittest

import wolf_demo_sweep_compare as sweep


class DemoSweepTest(unittest.TestCase):
    def test_disabled_occupancy_removes_even_inherited_zero(self):
        inherited = {"GDWOLF_DEMO_OCCUPANCY": "0", "PATH": "/bin"}
        self.assertEqual(sweep.occupancy_environment(inherited, False), {"PATH": "/bin"})
        self.assertEqual(sweep.occupancy_environment(inherited, True),
                         {"PATH": "/bin", "GDWOLF_DEMO_OCCUPANCY": "1"})
        self.assertEqual(inherited["GDWOLF_DEMO_OCCUPANCY"], "0")

    def test_missing_malformed_and_non_object_results(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "result.json"
            for data in (None, "{", "[]", "null"):
                if data is not None:
                    path.write_text(data)
                result, error = sweep.read_result(path)
                self.assertIsNone(result)
                self.assertTrue(error)
            sweep.write_json(path, {"status": "mismatch", "matched_commands": 4})
            self.assertEqual(sweep.read_result(path), ({"status": "mismatch", "matched_commands": 4}, None))

    def test_coverage_excludes_initial_and_unmatched_states(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "port.jsonl"
            records = []
            for command, kind in [(-1, 9), (0, 1), (1, 2), (2, 8)]:
                records.append({"command": command, "state": {
                    "player": {"x": 65536 * (command + 2), "y": 65536, "angle": 10},
                    "actors": [{"kind": kind, "state": 3, "active": kind == 2}],
                    "doors": [{"action": 0}], "weapon": {"type": 1, "shots": 1},
                    "damage": 2, "sound": {"playing": 0},
                }})
            path.write_text("".join(json.dumps(record) + "\n" for record in records))
            coverage = sweep.matched_coverage(path, 2)
            self.assertEqual(coverage["samples"], 2)
            self.assertEqual(coverage["actor_kinds"], [1, 2])
            self.assertEqual(coverage["active_actor_kinds"], [2])
            self.assertEqual(coverage["player_tiles"], [(2, 1), (3, 1)])
            self.assertEqual(coverage["shots"], 2)
            self.assertEqual(coverage["player_damage"], 4)


if __name__ == "__main__":
    unittest.main()
