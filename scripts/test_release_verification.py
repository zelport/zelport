import hashlib
import importlib.util
from pathlib import Path
import re
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("verify_release_assets.py")
SPEC = importlib.util.spec_from_file_location("verify_release_assets", SCRIPT)
VERIFY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(VERIFY)


class ReleaseAssetVerificationTest(unittest.TestCase):
    version = "0.10.1"

    def bundle(self, root: Path, published: bool = False) -> Path:
        bundle = root / "bundle"
        bundle.mkdir()
        lines = []
        for name in VERIFY.payload_names(self.version):
            if name == "PKGBUILD":
                content = f"pkgver={self.version}\n".encode()
            elif name.endswith(".SRCINFO"):
                content = f"pkgver = {self.version}\n".encode()
            else:
                content = name.encode()
            (bundle / name).write_bytes(content)
            lines.append(f"{hashlib.sha256(content).hexdigest()}  {name}")
        (bundle / "SHA256SUMS").write_text("\n".join(lines) + "\n")
        if not published:
            formula = bundle / "Formula"
            formula.mkdir()
            (formula / "zelport.rb").write_text(f'  version "{self.version}"\n')
        return bundle

    def test_accepts_exact_local_and_published_assets(self):
        with tempfile.TemporaryDirectory() as temporary:
            VERIFY.verify_assets(self.bundle(Path(temporary)), self.version)
        with tempfile.TemporaryDirectory() as temporary:
            VERIFY.verify_assets(
                self.bundle(Path(temporary), published=True),
                self.version,
                published=True,
            )

    def test_rejects_extra_asset_and_checksum_mismatch(self):
        with tempfile.TemporaryDirectory() as temporary:
            bundle = self.bundle(Path(temporary))
            (bundle / "unexpected").write_text("no")
            with self.assertRaisesRegex(ValueError, "asset set differs"):
                VERIFY.verify_assets(bundle, self.version)
            (bundle / "unexpected").unlink()
            (bundle / "unexpected-directory").mkdir()
            with self.assertRaisesRegex(ValueError, "asset set differs"):
                VERIFY.verify_assets(bundle, self.version)
            (bundle / "unexpected-directory").rmdir()
            (bundle / "PKGBUILD").write_text(f"pkgver={self.version}\nchanged\n")
            with self.assertRaisesRegex(ValueError, "checksum mismatch"):
                VERIFY.verify_assets(bundle, self.version)

    def test_rejects_invalid_or_mismatched_versions(self):
        with tempfile.TemporaryDirectory() as temporary:
            bundle = self.bundle(Path(temporary))
            with self.assertRaisesRegex(ValueError, "stable numeric SemVer"):
                VERIFY.verify_assets(bundle, "v0.10.1")
            (bundle / "PKGBUILD").write_text("pkgver=0.10.2\n")
            digest = hashlib.sha256((bundle / "PKGBUILD").read_bytes()).hexdigest()
            manifest = (bundle / "SHA256SUMS").read_text()
            manifest = re.sub(
                r"[0-9a-f]{64}  PKGBUILD",
                f"{digest}  PKGBUILD",
                manifest,
            )
            (bundle / "SHA256SUMS").write_text(manifest)
            with self.assertRaisesRegex(ValueError, "PKGBUILD version"):
                VERIFY.verify_assets(bundle, self.version)


if __name__ == "__main__":
    unittest.main()
