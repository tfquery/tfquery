# Installation

This guide covers all supported ways to install tfquery and related documentation (man pages and TLDR pages).

## Homebrew (macOS and Linux)

The simplest way to install on macOS/Linux with Homebrew:

```bash
brew tap tfquery/tfquery
brew install tfquery
```

Notes:
- The Homebrew formula installs man pages. Try: `man tfquery`, `man tfquery-sq`.

## Debian/Ubuntu (.deb package)

Download the latest .deb release packages for your platform from the [releases page](https://github.com/tfquery/tfquery/releases).

Before installing downloaded artifacts, verify signatures using [Signing and
verification](signing.md).

```bash
# Example for amd64; adjust the filename for the architecture and release you
# downloaded.
sudo dpkg -i /path/to/tfquery_1.7.1_amd64.deb
```

Notes:
- The Debian package installs man pages under `/usr/share/man/man1`.

## Pre-built tarball (manual install)

Download the latest release for your platform from the [releases page](https://github.com/tfquery/tfquery/releases).

Before installing downloaded artifacts, verify signatures using [Signing and
verification](signing.md).

Extract and move the binary to your PATH:

```bash
tar xvzf tfquery_*.tar.gz
sudo mv tfquery /usr/local/bin
```

### Install man pages (from tarball)

The tarball includes manual pages under `docs/man/share/man1`, but they aren’t auto-installed. If you’re unsure of your system’s man search paths, see the references at the end of this section.

#### Linux

1) Find a suitable man directory (check your search path):

```bash
manpath
# or show where a page would be read from
man -w printf || true
```

Common choices are `/usr/local/share/man` (preferred for local installs) or `/usr/share/man`.

2) Install the pages (adjust MANPREFIX if needed):

```bash
MANPREFIX=/usr/local/share/man
sudo install -d "$MANPREFIX/man1" "$MANPREFIX/man7"
sudo install -m 0644 docs/man/share/man1/* "$MANPREFIX/man1/"
sudo install -m 0644 docs/man/share/man7/* "$MANPREFIX/man7/"
# Update the whatis database (optional, man-db systems)
sudo mandb || true
```

#### macOS

1) Check your man path and pick a prefix that exists in it:

```bash
manpath | tr ':' '\n'
```

On Intel macs, `/usr/local/share/man` is common; on Apple Silicon with Homebrew, `/opt/homebrew/share/man` is typical.

2) Install the pages (adjust MANPREFIX if needed):

```bash
MANPREFIX=/usr/local/share/man   # or /opt/homebrew/share/man
sudo install -d "$MANPREFIX/man1" "$MANPREFIX/man7"
sudo install -m 0644 docs/man/share/man1/* "$MANPREFIX/man1/"
sudo install -m 0644 docs/man/share/man7/* "$MANPREFIX/man7/"
# Rebuild macOS whatis database (optional)
sudo /usr/libexec/makewhatis "$MANPREFIX" || true
```

#### Windows

- Native Windows doesn’t ship a `man` viewer. If you use WSL, follow the Linux steps inside your distro. If you use MSYS2/Cygwin/Git Bash, use the shell’s own `manpath` and copy into that environment’s `/usr/share/man` or `/usr/local/share/man`.

#### Verify installation

```bash
man -w tfquery     # shows the file path if the page is found
man tfquery        # opens the main tfquery page
man tfquery-sq     # opens the sq subcommand page
```

#### Helpful references (how man finds pages)

- Linux manpath(1): https://man7.org/linux/man-pages/man1/manpath.1.html
- Linux man(1): https://man7.org/linux/man-pages/man1/man.1.html
- macOS manpath(1): https://www.manpagez.com/man/1/manpath/
- macOS makewhatis(8): https://www.manpagez.com/man/8/makewhatis/

### Install TLDR pages (from tarball)

Tip: Most tfquery query commands support `--tldr` to show a short usage page, for
example: `tfquery oq --tldr`. This requires a TLDR client to be installed and the
tfquery pages to be available to that client (either via upstream pages or local
pages configured as below).

The tarball also includes TLDR pages under `docs/tldr`. These are compatible with common TLDR clients (e.g., `tldr`, `tlrc`), but clients don’t auto-load third-party pages by default. You have two options:

- Contribute these pages upstream to the community repo so they’re fetched like any other page; or
- Point your TLDR client to a local pages directory or mirror that includes `docs/tldr`.

Below are general approaches for popular clients. Consult your client’s docs for the most accurate, up-to-date details.

#### Prerequisite: install a TLDR client

- Linux/macOS (Node.js client): `npm install -g tldr`
- Linux/macOS (Rust client): `brew install tlrc` (macOS/Homebrew) or `cargo install tlrc` (Rust toolchain)

#### Linux

1) Choose a pages folder. For many clients, a reasonable local path is:

```bash
TLDRPAGES="$HOME/.local/share/tldr/pages"
mkdir -p "$TLDRPAGES"
cp -a docs/tldr/* "$TLDRPAGES/"
```

2) Point your client at that folder:

- For `tlrc`, set a config file (example `~/.config/tlrc/config.toml`):

```
[directories]
pages = ["$HOME/.local/share/tldr/pages"]
```

- For the Node.js `tldr` client, you can set `TLDR_CACHE_DIR` to a directory containing `pages/` (client behavior may vary by version):

```bash
export TLDR_CACHE_DIR="$HOME/.cache/tldr"
mkdir -p "$TLDR_CACHE_DIR/pages"
cp -a docs/tldr/* "$TLDR_CACHE_DIR/pages/"
```

Then try:

```bash
tldr tfquery
tldr tfquery sq
```

#### macOS

The steps are the same as Linux; adjust any paths for Homebrew conventions. For example, with `tlrc` installed via Homebrew, configure `~/.config/tlrc/config.toml` as above.

#### Windows

- WSL: follow the Linux steps inside your distro.
- MSYS2/Cygwin/Git Bash: use the shell’s HOME and XDG paths (e.g., `~/.local/share/tldr/pages`), then configure your client similarly.

#### Helpful references (tldr clients)

- TLDR community pages: https://github.com/tldr-pages/tldr
- Node.js `tldr` client: https://github.com/tldr-pages/tldr-node-client
- Rust `tlrc` client: https://github.com/tldr-pages/tlrc

## Build from source

```bash
git clone https://github.com/tfquery/tfquery.git
cd tfquery && go build -o tfquery
```

## Build with GoReleaser

```bash
goreleaser build --snapshot --clean --single-target
```
