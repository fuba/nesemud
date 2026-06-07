# Progress Update (2026-06-08): Edge Race 9面クリアと開発回顧

## 9面クリアの位置づけ
`edgerace.nes` は、`nes-race-danmaku` の開発版 ROM として継続的に確認してきた対象である。
2026-04-07 の closeout では owned-evidence による 240 frame / 2400 frame の再確認で action item が `0` になり、描画停止・CPU停止・音声欠落のような明確な実行上の問題は出ていないことを確認していた。

今回、実プレイ上の節目として「9面クリア」まで到達した。
ここでいうゲームクリアは、公式なエンディング到達や完全互換性の宣言ではなく、このプロジェクトで実用上の到達点として置いた「9面クリア」を指す。
それでも、単に数百 frame 動くこととは意味が違う。長時間の入力、スクロール、敵や弾の更新、当たり判定、音声、ストリーミング、操作APIが破綻せず最後まで使えることを、人間のプレイを通じて確認できた節目である。

## 開発の試行錯誤の流れ
最初の重点は、エミュレータ単体の正しさよりも「観察できる実行環境」を作ることだった。
`nesd` を daemon として起動し、HTTP API から ROM load、reset、input、state、memory inspection を操作できるようにし、HLS と WebRTC で映像・音声を外から見られるようにした。
これにより、テストだけでなく、実際のゲーム画面とログを往復しながら問題を切り分けられるようになった。

その後の互換性改善は、owned-ROM evidence と実ROMで見える症状を起点に、CPU、PPU、APU、mapper の各層へ掘り下げる流れになった。
停止、単色画面、音声なし、画面崩れ、長時間実行後の破綻といった現象を、直接の見た目だけで判断せず、CPU pause metadata、non-uniform frame observation、mapper hotspot、stream stats などに分解して記録した。
症状を小さく分けたことで、「どのゲームが動かないか」ではなく「どのタイミング・バンク・割り込み・描画状態が壊れるか」として扱えるようになった。

特に大きかったのは、割り込みと mapper の扱いを実ゲーム寄りに直していった段階である。
NMI/IRQ の遅延、CLI/SEI まわりのサンプリング、MMC3 の IRQ clock gating、mapper4 PRG-RAM control、mapper206 の bank math、mapper25 の IRQ ACK などは、単体仕様としては細かいが、実ゲームでは進行不能や不自然な画面更新に直結する。
それぞれを regression test に落とし込み、同じ問題が戻らない形にした。

PPU 側では、frame boundary、vblank entry、odd-frame pre-render dot skip、scanline split state、背景不透明バッファの再利用などを詰めた。
ここは見た目の破綻に直結するため、単にフレームが出るだけでは足りない。
長時間実行やスクロール中にも、過去フレームの状態が残らず、分割スクロールや描画タイミングが大きく崩れないことを重視した。

最後に、プレイと記録のための道具を追加した。
入力シミュレーション endpoint と trace snapshot により、候補入力列を live core から分離した clone 上で評価できるようにした。
さらに `nesd-record-hls` により、HLS stream を MP4 と sidecar JSON、累積 `manifest.json` として保存できるようにした。
これで、単発の目視確認ではなく、プレイ時点の HLS URL、metadata snapshot、開始・終了時刻、recording reason を残せるようになった。

## 技術的に効いた改善
今回の到達に効いたのは、大きな一発の修正ではなく、観察、仮説、最小修正、回帰テスト化を何度も回したことだった。

- daemon/API/HLS/WebRTC により、実行状態を外から操作・観察できるようにした。
- validation suite と owned-evidence により、実ROMの異常を定量的に拾えるようにした。
- CPU pause、uniform frame、audio samples、mapper hotspot を記録し、症状の分類を可能にした。
- IRQ、NMI、mapper banking、PPU timing の細かいズレを regression test として固定した。
- 入力シミュレーションと HLS 録画により、プレイ・探索・証跡化を同じ環境で扱えるようにした。

これらはそれぞれ単独では地味だが、組み合わさることで「動いたように見える」状態から「最後まで遊んで確認できる」状態へ近づけた。
9面クリアは、その積み上げが実プレイの形で確認できたという意味を持つ。

## 最終到達点
2026-06-08 時点で、`Edge Race` はこのプロジェクトの実用上の節目である9面クリアまで到達した。
これは `nesemud` が完全な NES 互換エミュレータになったという宣言ではない。
未監査の sprite timing、mapper edge cases、APU detail、違法 opcode timing などはまだ将来の hardening 項目として残る。

一方で、今回の到達は、プロジェクトの目的に対して重要な確認になった。
APIで操作し、streamingで見て、実ROMを走らせ、必要なら入力をシミュレーションし、結果を録画と metadata で残せる。
その一連の流れが、実際のゲーム進行という長い経路を通って破綻しなかった。

## 今後に残す任意課題
このフェーズでは、以下は game clear 到達の blocker ではなく、将来の品質向上として扱う。

- conformance ROM による IRQ/BRK/RTI edge-case timing の追加監査。
- sprite evaluation/rendering と split/HUD 系タイトルでの PPU timing 監査。
- APU frame sequencer、envelope、length/sweep、mixing の精度確認。
- mapper 5/23/25/33/66/75/87/88/206 の実タイトル・テストROMでの追加検証。
- HLS 録画 manifest と入力 trace を組み合わせた、再現性の高いプレイ証跡の蓄積。

## 関連レポート
- `doc/progress-2026-04-07-runtime-refresh-and-closeout.md`
- `doc/master-checklist.md`
- `doc/development-schedule.md`
