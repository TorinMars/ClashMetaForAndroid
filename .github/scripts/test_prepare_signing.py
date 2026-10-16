import codecs
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("prepare_signing.py")
KEYS = ("SIGNING_STORE_PASSWORD", "SIGNING_KEY_ALIAS", "SIGNING_KEY_PASSWORD")


class PrepareSigningTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.environment = os.environ.copy()
        for key in KEYS:
            self.environment.pop(key, None)
        self.environment["GITHUB_OUTPUT"] = str(self.root / "output")
        (self.root / "release.keystore").touch()

    def run_script(self, *args, **values):
        return subprocess.run(
            [sys.executable, str(SCRIPT), *args], cwd=self.root,
            env={**self.environment, **values}, capture_output=True, text=True,
        )

    def test_absent_optional_secrets_leave_debug_signing_available(self):
        result = self.run_script()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.root / "signing.properties").exists())
        self.assertEqual((self.root / "output").read_text(), "configured=false\n")

    def test_absent_required_secrets_fail_before_building(self):
        result = self.run_script("--required")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SIGNING_STORE_PASSWORD", result.stderr)
        self.assertFalse((self.root / "signing.properties").exists())

    def test_partial_secrets_fail_without_exposing_values(self):
        secret = "dummy-secret-do-not-print"
        result = self.run_script(SIGNING_STORE_PASSWORD=secret)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SIGNING_KEY_ALIAS", result.stderr)
        self.assertNotIn(secret, result.stdout + result.stderr)
        self.assertFalse((self.root / "signing.properties").exists())

    def test_complete_secrets_create_private_java_properties_without_logging_them(self):
        values = dict(zip(KEYS, (" dummy\\password=caf\u00e9\n", "dummy-alias", "dummy-key-password")))
        result = self.run_script(**values)
        self.assertEqual(result.returncode, 0, result.stderr)
        settings = self.root / "signing.properties"
        decoded = {
            key: codecs.decode(value, "unicode_escape")
            for key, value in (line.split("=", 1) for line in settings.read_text().splitlines())
        }
        self.assertEqual(decoded["keystore.password"], values[KEYS[0]])
        self.assertEqual(decoded["key.alias"], values[KEYS[1]])
        self.assertEqual(decoded["key.password"], values[KEYS[2]])
        self.assertEqual(settings.stat().st_mode & 0o777, 0o600)
        self.assertEqual((self.root / "output").read_text(), "configured=true\n")
        for value in values.values():
            self.assertNotIn(value, result.stdout + result.stderr)

    def test_existing_settings_are_preserved(self):
        settings = self.root / "signing.properties"
        settings.write_text("existing settings")
        result = self.run_script(**dict(zip(KEYS, ("dummy-store", "dummy-alias", "dummy-key"))))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(settings.read_text(), "existing settings")


if __name__ == "__main__":
    unittest.main()
