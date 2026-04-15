<a id="document-top"></a>

[![License][license-shield]][license-url]

<br />
<div align="center">
  <a href="https://github.com/Edward-Lucas/Threadix">
    <img src="icons/logo.png" alt="Threadix Logo">
  </a>

<h3 align="center">Threadix</h3>

  <p align="center">
    A Windows thread and process manager with a Fyne-based GUI.
    <br />
    <br />
    <a href="#getting-started"><strong>Get started quickly »</strong></a>
    <br />
    <br />
    <a href="https://github.com/Edward-Lucas/Threadix/issues/new?labels=bug">Report Bug</a>
    ·
    <a href="https://github.com/Edward-Lucas/Threadix/issues/new?labels=enhancement">Request Feature</a>
  </p>
</div>

<details>
  <summary>Table of Contents</summary>
  <ol>
    <li><a href="#about-the-project">About The Project</a></li>
    <li><a href="#features">Features</a></li>
    <li><a href="#getting-started">Getting Started</a></li>
    <li><a href="#building">Building</a></li>
    <li><a href="#license">License</a></li>
  </ol>
</details>

## About The Project

Threadix is a Windows utility for managing process priority, CPU affinity, and thread grouping.
It uses a Fyne GUI for process selection, priority changes, and affinity management while applying rules in the background.

### What this repository contains

- `backend/` — Go-based backend and GUI application
- `backend/build.bat` — build script with icon resource support
- `backend/build_no_console.bat` — GUI-only build script without console window
- `backend/resources/` — application icon and optional Korean font resource

<p align="right">(<a href="#document-top">back to top</a>)</p>

## Features

- Manage running processes on Windows
- View and change process priority classes
- Move processes between thread groups with specific core affinity
- Restore the current process affinity to the full system mask on startup
- Tray menu support and close-to-tray behavior
- Windows GUI build scripts with embedded icon support

<p align="right">(<a href="#document-top">back to top</a>)</p>

## Getting Started

To run the application from source:

```powershell
cd backend
go run .
```

For a release-style Windows executable with GUI subsystem:

```powershell
cd backend
build_no_console.bat
```

If you want a standard build instead:

```powershell
cd backend
go build -buildvcs=false -ldflags "-H=windowsgui" -o threadix.exe .
```

> Note: `resources/malgun.ttf` is included as an optional font resource, but the application will also attempt to use the system-installed Malgun Gothic font if available.

<p align="right">(<a href="#document-top">back to top</a>)</p>

## Building

The provided build scripts handle icon embedding and creating a Windows GUI executable.

- `build.bat` — builds `threadix.exe` with icon and GUI subsystem
- `build_no_console.bat` — builds without a console window

If you prefer manual build steps, use `go build` with the `-ldflags "-H=windowsgui"` option.

<p align="right">(<a href="#document-top">back to top</a>)</p>

## License

This project follows the license defined in `process-governor-release/LICENSE`.

<p align="right">(<a href="#document-top">back to top</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->

[license-shield]: https://img.shields.io/github/license/Edward-Lucas/Threadix.svg?style=for-the-badge

[license-url]: https://github.com/Edward-Lucas/Threadix/blob/main/process-governor-release/LICENSE

[issues-url]: https://github.com/SystemXFiles/process-governor/issues

[license-shield]: https://img.shields.io/github/license/SystemXFiles/process-governor.svg?style=for-the-badge

[license-url]: https://github.com/SystemXFiles/process-governor/blob/master/LICENSE
