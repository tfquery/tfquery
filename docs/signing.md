# Release Signing With GPG

This project uses GPG detached signatures for release artifacts.

Why sign?

- Verifies integrity: users can confirm the downloaded artifact matches what was
	released.
- Verifies authenticity: users can confirm the artifact was signed by the
	project release key.

How releases are signed

- CI imports the private key from `GPG_PRIVATE_KEY`.
- GoReleaser signs all artifacts with `gpg --detach-sign --armor`.
- Detached signatures (`.sig`) are uploaded with the release artifacts.
- CI verifies the generated signatures against `KEYS` before release completion.

## Public key

- Public key file: `KEYS`.
- Canonical source: `https://raw.githubusercontent.com/tfquery/tfquery/master/KEYS`.
- Current release key fingerprint:

```
F36D 6C7C 87B2 C6EC 7BA9 F561 D842 C5AA 7A0F 8965
```

You can inspect the fingerprint locally:

```bash
gpg --show-keys --with-fingerprint KEYS
```

## Verify a release artifact

```bash
TAG=v1.7.0
ARCHIVE="tfquery_${TAG}_linux_x86_64.tar.gz"

curl -fL "https://github.com/tfquery/tfquery/releases/download/${TAG}/${ARCHIVE}" -o "${ARCHIVE}"
curl -fL "https://github.com/tfquery/tfquery/releases/download/${TAG}/${ARCHIVE}.sig" -o "${ARCHIVE}.sig"
curl -fL "https://raw.githubusercontent.com/tfquery/tfquery/master/KEYS" -o KEYS

gpg --import KEYS
gpg --verify "${ARCHIVE}.sig" "${ARCHIVE}"
```

Expected output includes:

```
gpg: Good signature from "tfquery <staranto@gmail.com>"
```

## Verify all signatures in a release directory

```bash
for sig in ./*.sig; do
	artifact="${sig%.sig}"
	gpg --verify "$sig" "$artifact"
done
```
