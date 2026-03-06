# ROM Test Assets

Place validation ROM files under `tests/roms/`.

Expected naming conventions used by suite filters:
- nestest: filenames containing `nestest` or `cpu`
- blargg-cpu: filenames containing `blargg` and `cpu`
- ppu: filenames containing `ppu`
- apu: filenames containing `apu`
- mapper: filenames containing `mapper` or `mmc` or `uxrom` or `cnrom`

Examples:
- `tests/roms/nestest.nes`
- `tests/roms/blargg_cpu_instr_test.nes`
- `tests/roms/ppu_vbl_nmi.nes`
- `tests/roms/apu_len_ctr.nes`
- `tests/roms/mmc3_irq_test.nes`
