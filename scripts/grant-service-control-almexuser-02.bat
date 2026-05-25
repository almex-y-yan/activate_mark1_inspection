@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
set "PS1=%SCRIPT_DIR%grant-service-control.ps1"
set "BACKUP=%SCRIPT_DIR%service-acl-backup.json"
set "USER_NAME=%COMPUTERNAME%\almexuser-02"

if not exist "%PS1%" (
  echo [ERROR] スクリプトが見つかりません: "%PS1%"
  exit /b 1
)

echo [INFO] 対象ユーザー: %USER_NAME%
echo [INFO] 実行モード: grant
echo [INFO] バックアップ: %BACKUP%

powershell -NoProfile -ExecutionPolicy Bypass ^
  -File "%PS1%" ^
  -User "%USER_NAME%" ^
  -Mode grant ^
  -BackupFile "%BACKUP%"

set "EXIT_CODE=%ERRORLEVEL%"
if not "%EXIT_CODE%"=="0" (
  echo [ERROR] 実行失敗 (exit=%EXIT_CODE%)
  exit /b %EXIT_CODE%
)

echo [INFO] 完了しました
exit /b 0
