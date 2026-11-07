# Desktop app

LCARS includes a native desktop entry point built with [Wails v3](https://v3.wails.io/).
It displays the same embedded canvas and reads the same content directories as
`lcars serve`. It runs the content store in the app process and opens no HTTP
listener. Closing the window quits the app and stops its content watcher and any
scheduled scripts.

The desktop code is a separate Go module in `desktop/`. Building or testing the
CLI does not require Wails or native webview development libraries. Both modules
use the pinned Go toolchain in `mise.toml`.

## Build and run

Install the [Wails platform dependencies](https://v3.wails.io/getting-started/installation/):

- Linux: a C compiler, `pkg-config`, GTK4 and WebKitGTK 6.0 development libraries.
  On Ubuntu 24.04: `sudo apt install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`.
  On Arch: `sudo pacman -S --needed base-devel pkgconf gtk4 webkitgtk-6.0`.
- macOS: Xcode command line tools (`xcode-select --install`).
- Windows: the WebView2 runtime (normally already installed).

From the repository root:

```sh
mise install
mise run desktop
./desktop/lcars-desktop --data ./examples/content
```

Or build directly:

```sh
cd desktop
go build -o lcars-desktop .
./lcars-desktop --data ../examples/content
```

On Windows, build `lcars-desktop.exe` and run it from PowerShell. To build without
a console window, use `go build -ldflags "-H windowsgui" -o lcars-desktop.exe .`.
The Wails CLI and a JavaScript build step are not needed; the Go module pins Wails
and embeds the existing frontend. `mise run desktop:run` opens the example canvas.

## Your content

`--data DIR` opens an existing directory and refuses a missing directory. Without
`--data`, LCARS creates and opens a persistent private directory:

- Linux: `$XDG_CONFIG_HOME/lcars/content`, or `~/.config/lcars/content`.
- macOS: `~/Library/Application Support/lcars/content`.
- Windows: `%AppData%\lcars\content`.

The window title shows the full directory path. Add section folders and panels
there using the [authoring guide](../skills/lcars-authoring/references/guide.md).
The app polls state every two seconds while visible, so edits appear without a
restart. Polling avoids platform differences in webview response streaming.

Scheduled scripts are disabled by default. Pass `--run-scripts` only for trusted
content; this uses the same runner as the server, with a 30-second timeout and
four concurrent scripts. See the [reference](reference.md) for the script
publication contract. For advanced interpreter/environment configuration, use
`lcars serve` with its full set of script options.

## Development and validation

```sh
mise run test
mise run desktop:test
```

Root tests cover the desktop handler, initial navigation, polling, file updates,
host checks, and component isolation. Desktop tests check argument and directory
handling before the native window starts. CI builds and tests the desktop module
on Linux, macOS, and Windows with their native libraries. The ordinary CLI build
and release command continue to package the CLI; desktop installers, app bundles,
code signing, and auto-updates are not provided yet.
