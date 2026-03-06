# NES Recorder Operations Manual

## 1. 概要
`nes_recorder` は NES エミュレータを daemon として常駐させ、外部から HTTP API で制御し、映像/音声を HLS で配信するアプリケーションです。

設計上の前提:
- 操作はすべて API 経由
- ログはファイル出力
- Docker 実行時はコンテナ内 `ffmpeg` を使用（ホスト側 `ffmpeg` 不要）

## 2. 構成
- daemon: `cmd/nesd`
- emulator core: `internal/nes`
- API: `internal/api`
- HLS streamer: `internal/streaming`
- validation: `cmd/nes-validate`, `internal/validation`

既定ポート:
- `:18080`

既定エンドポイント:
- API: `http://127.0.0.1:18080/v1/...`
- HLS: `http://127.0.0.1:18080/hls/index.m3u8`

## 3. 必要環境
ローカル実行:
- Go 1.24+
- `ffmpeg` が PATH 上で実行可能

Docker 実行:
- Docker / Docker Compose
- Linux で `network_mode: host` が使えること

## 4. 起動と停止
### 4.1 Docker（推奨）
起動:
```bash
docker compose up -d --build
```

停止:
```bash
docker compose down
```

確認:
```bash
curl -sS http://127.0.0.1:18080/v1/state
curl -sS http://127.0.0.1:18080/v1/stream/stats
ls -la runtime/hls
```

### 4.2 ローカル直接起動
前景起動:
```bash
go run ./cmd/nesd serve --config ./config.example.json
```

擬似 daemon 起動:
```bash
go run ./cmd/nesd daemon --config ./config.example.json
```

## 5. 設定
設定ファイル JSON:
```json
{
  "listen_addr": ":18080",
  "log_file": "./nesd.log",
  "hls_dir": "./hls"
}
```

各項目:
- `listen_addr`: API/HLS を待ち受けるアドレス
- `log_file`: daemon ログファイル
- `hls_dir`: HLS 生成先ディレクトリ

ホットリロード:
- `SIGHUP` で設定再読込（`listen_addr` の再 bind はしない）

## 6. API リファレンス
OpenAPI（簡易）:
```bash
curl -sS http://127.0.0.1:18080/v1/openapi.json
```

### 6.1 状態
- `GET /v1/state`
- 実行状態、CPU/PPU/APU の主要値、`rom_loaded`、`replay_active` を返す

### 6.2 ROM 読み込み
- `POST /v1/rom/load`
- body:
```json
{"path":"/absolute/or/container/path/game.nes"}
```

### 6.3 制御
- `POST /v1/control/reset`
- `POST /v1/control/pause`
- `POST /v1/control/resume`

### 6.4 FM2 replay
- `POST /v1/replay/fm2`
- `path` または `content` を指定
- 投入時に内部状態を reset して replay を先頭から開始

例（path 指定）:
```bash
curl -sS -X POST http://127.0.0.1:18080/v1/replay/fm2 \
  -H 'Content-Type: application/json' \
  -d '{"path":"/tests/fm2/sample.fm2"}'
```

例（content 直接投入）:
```bash
curl -sS -X POST http://127.0.0.1:18080/v1/replay/fm2 \
  -H 'Content-Type: application/json' \
  -d '{"content":"version 3\n|0|........|........|\n"}'
```

### 6.5 メモリ参照/書込
- `GET /v1/memory/{addr}?len=N`
- `PUT /v1/memory/{addr}`

`PUT` body は `value` または `bytes`:
```json
{"value":171}
```
```json
{"bytes":[1,2,3,4]}
```

注意:
- `addr` は `0x10` のような 16進表記可
- 長さ/書込サイズは 1..4096 バイト制限

### 6.6 コントローラ入力
- `PUT /v1/input/player/1`
- `PUT /v1/input/player/2`

body:
```json
{
  "a": false,
  "b": false,
  "select": false,
  "start": false,
  "up": false,
  "down": false,
  "left": false,
  "right": false
}
```

### 6.7 RetroArch 互換コマンド
- `POST /v1/retroarch/command`
- `GET /v1/retroarch/command/list`

対応コマンド:
- `RESET`
- `SOFT_RESET`
- `PAUSE`
- `RESUME`
- `UNPAUSE`

指定方法:
- query `?cmd=RESET`
- JSON body `{"command":"RESET"}`
- plain text body `RESET`

### 6.8 検証 API
- `POST /v1/validate/replay`
- `POST /v1/validate/nestest`
- `POST /v1/validate/suite`

例:
```bash
curl -sS -X POST http://127.0.0.1:18080/v1/validate/suite \
  -H 'Content-Type: application/json' \
  -d '{"suite":"ppu","rom_dir":"/tests/roms","frames":3000}'
```

### 6.9 ストリーム統計
- `GET /v1/stream/stats`

主な値:
- `running`: streamer 稼働状態
- `written_frames`: ffmpeg へ送れたフレーム数
- `dropped_frames`: キュー逼迫で破棄したフレーム数
- `queue_depth`, `queue_capacity`: 内部キュー状態

## 7. HLS 配信
プレイリスト URL:
- `http://127.0.0.1:18080/hls/index.m3u8`

配信仕様（現行実装）:
- Video: 256x240, RGB24 入力, H.264 出力
- Audio: PCM S16LE stereo 48kHz 入力, AAC 出力
- HLS segment: 1秒, list size 5

検証:
```bash
curl -sS http://127.0.0.1:18080/v1/stream/stats
head -n 20 runtime/hls/index.m3u8
```

## 8. 実運用フロー
1. daemon 起動
2. `POST /v1/rom/load` で ROM 読込
3. 任意で `POST /v1/replay/fm2` 投入
4. `GET /v1/state` で状態確認
5. `GET /hls/index.m3u8` をプレイヤーで再生

## 9. 障害対応
### 9.1 HLS が生成されない
確認:
```bash
curl -sS http://127.0.0.1:18080/v1/stream/stats
tail -n 200 runtime/nesd.log
ls -la runtime/hls
```

観点:
- `written_frames` が増えているか
- `runtime/hls/index.m3u8` と `.ts` が作成されるか
- コンテナ内 `ffmpeg` が動作しているか

### 9.2 音が出ない
確認:
- セグメントに audio stream があるか
```bash
docker compose exec -T nesd ffprobe -v error -show_streams /data/hls/index0.ts
```
- `mean_volume` が `-inf` でないか
```bash
docker compose exec -T nesd sh -lc 'f=$(ls -1 /data/hls/*.ts | tail -n 1); ffmpeg -hide_banner -i "$f" -vn -af volumedetect -f null - 2>&1 | tail -n 20'
```

### 9.3 API が効かない
確認:
- daemon が起動中か
- ポート `18080` が他プロセスと競合していないか
- `GET /v1/state` が返るか

## 10. 検証・テスト
ユニットテスト:
```bash
go test ./...
```

互換検証CLI:
```bash
go run ./cmd/nes-validate --suite nestest --rom-dir ./tests/roms --frames 2000
go run ./cmd/nes-validate --suite blargg-cpu --rom-dir ./tests/roms --frames 3000
go run ./cmd/nes-validate --suite ppu --rom-dir ./tests/roms --frames 3000
go run ./cmd/nes-validate --suite apu --rom-dir ./tests/roms --frames 3000
go run ./cmd/nes-validate --suite mapper --rom-dir ./tests/roms --frames 3000
```

一括実行:
```bash
./scripts/run-compat-suites.sh ./tests/roms 3000
```

## 11. 現時点の制約
- 「完全互換」は未達。`doc/master-checklist.md` の未完了項目を参照。
- FM2 は入力フレーム列を対象にした簡易パーサ。複雑な拡張メタ情報は未対応。
- OpenAPI は簡易仕様。実装上の全エンドポイントは本書 6章を正とする。
