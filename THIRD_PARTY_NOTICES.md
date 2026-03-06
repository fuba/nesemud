# Third-Party Notices

このファイルは、`nes_recorder` の設計・運用で参照する外部ソフトウェア/仕様と、ライセンス上の取り扱い方針を整理したものです。

## 1. 本リポジトリのコードライセンス
- `nes_recorder` 本体コード: MIT
- ライセンステキスト: [`LICENSE`](./LICENSE)

## 2. バンドルされる実行依存
### 2.1 FFmpeg（Docker イメージ内）
- 用途: HLS 生成（video/audio エンコード）
- 取得方法: Docker build 中に Debian パッケージをインストール
- 重要: FFmpeg のライセンスはビルド構成で変わります（LGPL/GPL）

運用時の確認コマンド:
```bash
docker compose exec -T nesd ffmpeg -version
```

`configuration:` に `--enable-gpl` が含まれる場合は GPL 条件での配布を要する可能性があります。配布形態に応じて法務判断を行ってください。

## 3. 参照した仕様/互換インターフェース
### 3.1 RetroArch command インターフェース（互換 API）
- 本実装は command 名の互換を意図しており、`/v1/retroarch/command` を提供します。
- 互換対象の API 名/コマンド名を参照しているだけで、RetroArch ソースコードは本リポジトリに取り込んでいません。

### 3.2 FM2 replay 形式（入力ログ）
- 本実装は FM2 のフレーム入力行を読み取り、P1/P2 のボタン列を再生します。
- FM2 は互換フォーマットとして参照しており、外部実装コードの転載はしていません。

## 4. 互換検証ROM・アセットのライセンス
`tests/roms`・`tests/fm2` に配置する外部アセット（ROM、ログ、リプレイ）は、配布元ごとにライセンス条件が異なります。  
このリポジトリでは原則としてそれらを同梱せず、利用者が取得・配置する前提です。

注意点:
- テスト ROM を再配布する場合は、各配布元ライセンスに従ってください。
- 商用 ROM/ゲームデータは権利者許諾なしで配布しないでください。

## 5. ソース/参照先（ライセンス確認用）
- FFmpeg legal: https://ffmpeg.org/legal.html
- FFmpeg project: https://ffmpeg.org/
- RetroArch project: https://github.com/libretro/RetroArch
- FCEUX project (FM2 関連エコシステム): https://github.com/TASEmulators/fceux
- NESDev emulator tests overview: https://www.nesdev.org/wiki/Emulator_tests
- NES test ROM corpus mirror used by `scripts/fetch-test-roms.sh`: https://github.com/christopherpow/nes-test-roms

## 6. メンテナ向けポリシー
- 外部コードをコピーした場合は必ず出典 URL とライセンスを追記すること。
- 外部バイナリを配布アーティファクトに含める場合は、再配布条件（著作権表示・ソース開示など）を確認すること。
- このファイルをリリース前チェック項目に含め、差分があれば更新すること。
