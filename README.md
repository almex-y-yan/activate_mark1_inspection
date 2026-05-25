# ini更新 + サービス再起動 GUI

Go + ブラウザUIで、対象 `.ini` の `Com` を更新し、
対応サービスを再起動するツールです。

## 前提

- Windows 環境
- `ALMEXPATH` 環境変数が定義済み
- `ALMEXPATH\ini` 配下に対象ファイルが存在
- サービス停止/起動の実行権限があるユーザーで実行
  （運用ユーザーに事前付与。実行時のパスワード入力は不要）

## 起動方法

1. PowerShell / コマンドプロンプトを起動
2. ツール配置フォルダへ移動
3. 次を実行

```powershell
go run ./cmd/web
```

4. ブラウザで `http://127.0.0.1:18080` を開く

## `web.exe` のファイルバージョン

Windows の `web.exe` に表示されるファイルバージョンは
`cmd/web/versioninfo.json` で管理します。

1. `cmd/web/versioninfo.json` の `FileVersion` / `ProductVersion` を更新
2. `goversioninfo` をインストール
3. 次を実行

```powershell
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
go generate ./cmd/web
go build -o web.exe ./cmd/web
```

`go generate ./cmd/web` で `cmd/web/resource.syso` が再生成され、
次の `go build` で `web.exe` に埋め込まれます。

## 対象ファイルとサービス

- `almex_card_crl31.ini` -> `almdevcd7`
- `almex_iccard_irs270.ini` -> `almdevic2`
- `almex_iccard_nm43.ini` -> `almdevic5`

## 画面の使い方

- `Card対象` をチェックすると `Card Com` が必須
- `IRS対象` をチェックすると `DEVICE1 Com` が必須
- `DEVICE2 を使用` をチェックすると `DEVICE2 Com` が必須
- `NM43対象` をチェックすると `Nm43 Com` が必須
- 未選択の対象は処理されません

## IRS DEVICE2 の仕様

`almex_iccard_irs270.ini` の処理内容:

- `DEVICE1 Com`: `[DEVICE1]` の `Com` を更新
- `DEVICE2 を使用` が ON:
  - `[DEVICE2]` のコメント状態を解除
  - `[DEVICE2]` の `Com` を更新
- `DEVICE2 を使用` が OFF:
  - `[DEVICE2]` セクション全体をコメントアウト

`[DEVICE2]` コメントアウトは `;AUTO_OFF ` 接頭辞で管理します。

## 実行時の動作

1. 対象 `.ini` を更新判定
2. 変更がある場合のみバックアップ作成
   (`ALMEXPATH\ini\backup` フォルダ)
3. 対応サービス停止
4. `.ini` 保存
5. 対応サービス開始

失敗時は、保存済みの場合にバックアップから復元します。

サービス停止/起動は `sc` コマンドで実行します。

## 出荷検査ツール開始時のサービス制御

検証機（1号機基準）の確認結果に合わせて、
`出荷検査ツール開始` ではサービスを次の方針で制御します。

- 確認日: `2026/05/25`
- 確認者: `楊`
- 提供用 `web.exe` 格納場所:
  `G:\共有ドライブ\R&Dデバイス制御グループ\Mark1\自己診断起動ツール`

処理順は次の通りです。

1. `STOP` 対象サービスを停止
2. `START` 対象サービスを開始
3. `almdevic5` の開始成功時は 2 秒待機
4. `mark1_inspection.exe` を起動

現在の実装上の注意:

- `Card` / `IRS` / `NM43` は、画面のチェック状態に応じて
  `開始のみ` ユーザー制御します
- 上記3サービスは未選択でも `停止` は行います
- `almdevic5` (`NM43`) の開始に成功した場合は、
  起動安定化のため `2秒` 待機してから `mark1_inspection.exe` を起動します
- 一部サービスの停止/開始に失敗しても、
  ログを残した上で `mark1_inspection.exe` の起動は継続します

### STOP 対象

- `almfnclg` : ALMEX Common Function (SystemLog)
- `almhlpcd` : ALMEX Common Helper (Card)
- `almhlpld` : ALMEX Common Helper (LED)
- `almhlppr` : ALMEX Common Helper (Printer)
- `almhlpss` : ALMEX Common Helper (Sensor)
- `almhlpsd` : ALMEX Common Helper (Sound)
- `almhlptm` : ALMEX Common Helper (SystemTime)
- `texcashctl` : ALMEX TEX Helper (CashControl)
- `texct` : ALMEX TEX Helper (Controller)
- `texdt` : ALMEX TEX Helper (DBData)
- `texms` : ALMEX TEX Helper (DBMaster)
- `texmy` : ALMEX TEX Money
- `texpay` : ALMEX TEX Payment
- `texst` : ALMEX FIT-A Status
- `almfncky` : ALMEX Common Function (Key)
- `texcs` : ALMEX TEX Helper (Console)
- `almfncad` : ALMEX Common Function (Address)
- `almfncpc` : ALMEX Common Function (PC Monitoring)
- `almfncsc` : ALMEX Common Function (String Convert)
- `almdevpp1` : ALMEX Common Device (PPR FC1-QOPU)
- `almdevcm1` : ALMEX Common Device (Camera ATM-CAM)
- `almdevcl9` : ALMEX Common Device (Cashless PAX A35/GMO)
- `almdevca7` : ALMEX Common Device (CashMachine AD-XR)
- `almdevic2` : ALMEX Common Device (ICCard IRS-270)
- `almdevmx1` : ALMEX Common Device (MIX Board AP-2462/CashMachine CLX-V231)
- `almdevic5` : ALMEX Common Device (ICCard NM43)
- `almdevps1` : ALMEX Common Device (PrinterThermal NP-3911)
- `almdevqr6` : ALMEX Common Device (QR FC1-QOPU)
- `almdevsd1` : ALMEX Common Device (Sound Standard)
- `almdevhd1` : ALMEX Common Device (HID Standard)
- `almdevcd7` : ALMEX Common Device (Card CR-L31)

### START 対象

常に開始するサービス:

- `almdevcm1` : ALMEX Common Device (Camera ATM-CAM)
- `almdevcl9` : ALMEX Common Device (Cashless PAX A35/GMO)
- `almdevca7` : ALMEX Common Device (CashMachine AD-XR)
- `almdevmx1` : ALMEX Common Device (MIX Board AP-2462/CashMachine CLX-V231)
- `almdevps1` : ALMEX Common Device (PrinterThermal NP-3911)
- `almdevqr6` : ALMEX Common Device (QR FC1-QOPU)
- `almdevsd1` : ALMEX Common Device (Sound Standard)
- `almdevhd1` : ALMEX Common Device (HID Standard)

画面のチェック状態で開始を切り替えるサービス:

- `almdevcd7` : ALMEX Common Device (Card CR-L31)
- `almdevic2` : ALMEX Common Device (ICCard IRS-270)
- `almdevic5` : ALMEX Common Device (ICCard NM43)

### START しない対象

停止のみで再開始しないサービス:

- `almfncad` : ALMEX Common Function (Address)
- `almfncky` : ALMEX Common Function (Key)
- `almfncpc` : ALMEX Common Function (PC Monitoring)
- `almfncsc` : ALMEX Common Function (String Convert)
- `almfnclg` : ALMEX Common Function (SystemLog)
- `almhlpcd` : ALMEX Common Helper (Card)
- `almhlpld` : ALMEX Common Helper (LED)
- `almhlppr` : ALMEX Common Helper (Printer)
- `almhlpss` : ALMEX Common Helper (Sensor)
- `almhlpsd` : ALMEX Common Helper (Sound)
- `almhlptm` : ALMEX Common Helper (SystemTime)
- `almdevpp1` : ALMEX Common Device (PPR FC1-QOPU)
- `texcashctl` : ALMEX TEX Helper (CashControl)
- `texcs` : ALMEX TEX Helper (Console)
- `texct` : ALMEX TEX Helper (Controller)
- `texdt` : ALMEX TEX Helper (DBData)
- `texms` : ALMEX TEX Helper (DBMaster)
- `texmy` : ALMEX TEX Money
- `texpay` : ALMEX TEX Payment
- `texst` : ALMEX FIT-A Status

補足:

- `tex_mainui_mark1_bh.exe` は Windows Service ではないため、
  現在のサービス制御対象には含めていません
- `almdevcd7` は `STOP` 対象に追加済みです
- 検証機の状態が変わった場合は、この一覧も更新してください

## ログ出力先

- サーバーログは `ALMEXPATH\log\ini-web-tool.log` に出力されます

## API仕様

- `GET /api/state`: 現在値取得（クエリ）
- `POST /api/apply`: ini更新 + サービス再起動（コマンド）
- 返却JSONの `statusError` は **`true` が正常**

## 復旧手順

1. `ALMEXPATH\ini\backup` から対象 `.ini` の最新 `.bak` を特定
2. 元ファイル名へコピーして置き換え
3. 対象サービスを再起動

## 標準ユーザー向けサービス権限付与

サービスの開始/停止権限を事前に付与するために
`scripts/grant-service-control.ps1` を用意しています。

1. 管理者権限の PowerShell で実行
2. 付与対象ユーザーを指定して `grant` 実行

```powershell
.\scripts\grant-service-control.ps1 `
  -User "PCNAME\operator" `
  -Mode grant `
  -BackupFile ".\service-acl-backup.json"
```

`almexuser-02` 固定で実行する場合は、次のラッパーを使えます。

```bat
.\scripts\grant-service-control-almexuser-02.bat
```

復元する場合は次を実行します。

```bat
.\scripts\restore-service-control-almexuser-02.bat
```

- `-Mode backup` : 変更なしでバックアップだけ作成
- `-Mode restore`: `-BackupFile` から元のSDDLへ復元
