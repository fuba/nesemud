# Third-Party Notices

This file documents external software/specs referenced by `nesemud` and the licensing/compliance handling policy.

## 1. Project Code License
- `nesemud` source code: MIT
- License text: [`LICENSE`](./LICENSE)

## 2. Bundled Runtime Dependencies
### 2.1 FFmpeg (inside Docker image)
- Purpose: HLS generation (video/audio encoding)
- Installation: Debian package installed during Docker build
- Important: FFmpeg licensing can differ by build configuration (LGPL/GPL)

Check command:
```bash
docker compose exec -T nesd ffmpeg -version
```

If `configuration:` includes `--enable-gpl`, distribution may require GPL obligations depending on your distribution model. Confirm with legal/compliance review as needed.

## 3. Referenced Specs / Compatibility Interfaces
### 3.1 RetroArch command interface (compatibility API)
- This implementation provides `/v1/retroarch/command` for command-level compatibility.
- It references API/command naming conventions only; RetroArch source code is not vendored into this repository.

### 3.2 FM2 replay format (input log)
- This implementation parses FM2 frame input lines and replays P1/P2 button states.
- FM2 is used as a compatibility format reference; no external implementation code is copied.

## 4. Compatibility ROM / Asset Licensing
External assets under `tests/roms` and `tests/fm2` (ROMs, logs, replays) can have different licenses by upstream source.
This repository generally does not bundle them by default and assumes users fetch/place them themselves.

Notes:
- If you redistribute test ROMs, follow the original upstream license terms.
- Do not redistribute commercial ROM/game data without explicit rights holder permission.

## 5. Sources / References (for license verification)
- FFmpeg legal: https://ffmpeg.org/legal.html
- FFmpeg project: https://ffmpeg.org/
- RetroArch project: https://github.com/libretro/RetroArch
- FCEUX project (FM2 ecosystem): https://github.com/TASEmulators/fceux
- NESDev emulator tests overview: https://www.nesdev.org/wiki/Emulator_tests
- NES test ROM corpus mirror used by `scripts/fetch-test-roms.sh`: https://github.com/christopherpow/nes-test-roms

## 6. Maintainer Policy
- If external code is copied, always add source URL and license details.
- If external binaries are included in deliverables, verify redistribution obligations (copyright notice/source requirements).
- Include this file in pre-release checks and update it whenever dependencies/references change.
