# nes_recorder

Go でフルスクラッチ実装する、CLI/Docker 前提の NES daemon です。

主機能:
- daemon 常駐実行
- HTTP API 経由の完全操作（ROM/入力/リセット/メモリ参照など）
- FM2 replay 投入（投入時に reset）
- HLS 配信（映像+音声）
- 互換検証 CLI/API

## クイックスタート
```bash
docker compose up -d --build
curl -sS http://127.0.0.1:18080/v1/state
```

HLS:
- `http://127.0.0.1:18080/hls/index.m3u8`

停止:
```bash
docker compose down
```

## マニュアル
- 完全運用マニュアル: [`doc/OPERATIONS_MANUAL.md`](./doc/OPERATIONS_MANUAL.md)
- テストROM配置: [`doc/rom-test-assets.md`](./doc/rom-test-assets.md)
- 開発進捗チェックリスト: [`doc/master-checklist.md`](./doc/master-checklist.md)
- サードパーティ/ライセンス注意: [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md)

## ライセンス
- 本リポジトリのコード: MIT（[`LICENSE`](./LICENSE)）
- 外部ソフト/テストROM/仕様参照の扱い: [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md)
