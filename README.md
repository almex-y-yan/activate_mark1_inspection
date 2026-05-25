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
