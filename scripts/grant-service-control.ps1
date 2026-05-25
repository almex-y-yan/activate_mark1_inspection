param(
  [Parameter(Mandatory = $true)]
  [string]$User,

  [ValidateSet("grant", "restore", "backup")]
  [string]$Mode = "grant",

  [string]$BackupFile = ".\service-acl-backup.json",

  [string[]]$Services = @(
    "almfnclg",
    "almhlpcd",
    "almhlpld",
    "almhlppr",
    "almhlpss",
    "almhlpsd",
    "almhlptm",
    "texcashctl",
    "texct",
    "texdt",
    "texms",
    "texmy",
    "texpay",
    "texst",
    "almfncky",
    "texcs",
    "almfncad",
    "almfncpc",
    "almfncsc",
    "almdevpp1",
    "almdevcm1",
    "almdevcl9",
    "almdevca7",
    "almdevic2",
    "almdevmx1",
    "almdevic5",
    "almdevps1",
    "almdevqr6",
    "almdevsd1",
    "almdevhd1",
    "almdevcd7"
  )
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Test-IsAdministrator {
  $current = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = New-Object Security.Principal.WindowsPrincipal($current)
  return $principal.IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator
  )
}

function Assert-Administrator {
  if (-not (Test-IsAdministrator)) {
    throw "管理者権限で実行してください。"
  }
}

function Resolve-UserSid {
  param([Parameter(Mandatory = $true)][string]$AccountName)

  $account = New-Object Security.Principal.NTAccount($AccountName)
  $sid = $account.Translate([Security.Principal.SecurityIdentifier])
  return $sid.Value
}

function Get-ServiceExists {
  param([Parameter(Mandatory = $true)][string]$ServiceName)

  try {
    Get-Service -Name $ServiceName -ErrorAction Stop | Out-Null
    return $true
  } catch {
    return $false
  }
}

function Get-ServiceSddl {
  param([Parameter(Mandatory = $true)][string]$ServiceName)

  $raw = & sc.exe sdshow $ServiceName 2>&1
  if ($LASTEXITCODE -ne 0) {
    throw "sdshow 失敗 ($ServiceName): $($raw -join ' ')"
  }

  $text = ($raw -join " ").Trim()
  $match = [regex]::Match($text, "(D:[^`r`n]+)")
  if (-not $match.Success) {
    throw "SDDL 解析失敗 ($ServiceName): $text"
  }
  return $match.Groups[1].Value
}

function Set-ServiceSddl {
  param(
    [Parameter(Mandatory = $true)][string]$ServiceName,
    [Parameter(Mandatory = $true)][string]$Sddl
  )

  $raw = & sc.exe sdset $ServiceName $Sddl 2>&1
  if ($LASTEXITCODE -ne 0) {
    throw "sdset 失敗 ($ServiceName): $($raw -join ' ')"
  }
}

function Add-AceToSddl {
  param(
    [Parameter(Mandatory = $true)][string]$Sddl,
    [Parameter(Mandatory = $true)][string]$Ace
  )

  if ($Sddl.Contains($Ace)) {
    return $Sddl
  }

  if ($Sddl.Contains("S:")) {
    $parts = $Sddl.Split("S:", 2)
    return "$($parts[0])$Ace" + "S:" + "$($parts[1])"
  }

  return "$Sddl$Ace"
}

function New-BackupData {
  param(
    [Parameter(Mandatory = $true)][string]$AccountName,
    [Parameter(Mandatory = $true)][string]$AccountSid,
    [Parameter(Mandatory = $true)][object[]]$Entries
  )

  return [pscustomobject]@{
    generatedAt = (Get-Date).ToString("o")
    machineName = $env:COMPUTERNAME
    accountName = $AccountName
    accountSid  = $AccountSid
    entries     = $Entries
  }
}

function Save-BackupFile {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][object]$Data
  )

  $dir = Split-Path -Parent $Path
  if ($dir -and -not (Test-Path -LiteralPath $dir)) {
    New-Item -Path $dir -ItemType Directory | Out-Null
  }
  $json = $Data | ConvertTo-Json -Depth 6
  Set-Content -LiteralPath $Path -Value $json -Encoding UTF8
}

function Load-BackupFile {
  param([Parameter(Mandatory = $true)][string]$Path)

  if (-not (Test-Path -LiteralPath $Path)) {
    throw "バックアップファイルが見つかりません: $Path"
  }
  return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
}

function Grant-ServiceControl {
  param(
    [Parameter(Mandatory = $true)][string]$AccountName,
    [Parameter(Mandatory = $true)][string]$AccountSid,
    [Parameter(Mandatory = $true)][string[]]$TargetServices,
    [Parameter(Mandatory = $true)][string]$Path
  )

  $uniqueServices = $TargetServices | Sort-Object -Unique
  $backupEntries = @()
  $ace = "(A;;RPWPLOLC;;;$AccountSid)"

  foreach ($serviceName in $uniqueServices) {
    if (-not (Get-ServiceExists -ServiceName $serviceName)) {
      Write-Host "[SKIP] サービスなし: $serviceName"
      continue
    }

    $currentSddl = Get-ServiceSddl -ServiceName $serviceName
    $backupEntries += [pscustomobject]@{
      serviceName = $serviceName
      sddl        = $currentSddl
    }

    $updatedSddl = Add-AceToSddl -Sddl $currentSddl -Ace $ace
    if ($updatedSddl -eq $currentSddl) {
      Write-Host "[SKIP] 付与済み: $serviceName"
      continue
    }

    Set-ServiceSddl -ServiceName $serviceName -Sddl $updatedSddl
    Write-Host "[OK] 権限付与: $serviceName"
  }

  $backupData = New-BackupData `
    -AccountName $AccountName `
    -AccountSid $AccountSid `
    -Entries $backupEntries
  Save-BackupFile -Path $Path -Data $backupData
  Write-Host "バックアップ保存: $Path"
}

function Backup-ServiceSddl {
  param(
    [Parameter(Mandatory = $true)][string]$AccountName,
    [Parameter(Mandatory = $true)][string]$AccountSid,
    [Parameter(Mandatory = $true)][string[]]$TargetServices,
    [Parameter(Mandatory = $true)][string]$Path
  )

  $uniqueServices = $TargetServices | Sort-Object -Unique
  $backupEntries = @()

  foreach ($serviceName in $uniqueServices) {
    if (-not (Get-ServiceExists -ServiceName $serviceName)) {
      Write-Host "[SKIP] サービスなし: $serviceName"
      continue
    }
    $currentSddl = Get-ServiceSddl -ServiceName $serviceName
    $backupEntries += [pscustomobject]@{
      serviceName = $serviceName
      sddl        = $currentSddl
    }
    Write-Host "[OK] バックアップ取得: $serviceName"
  }

  $backupData = New-BackupData `
    -AccountName $AccountName `
    -AccountSid $AccountSid `
    -Entries $backupEntries
  Save-BackupFile -Path $Path -Data $backupData
  Write-Host "バックアップ保存: $Path"
}

function Restore-ServiceSddl {
  param([Parameter(Mandatory = $true)][string]$Path)

  $backup = Load-BackupFile -Path $Path
  if (-not $backup.entries) {
    throw "バックアップに entries がありません: $Path"
  }

  foreach ($entry in $backup.entries) {
    $serviceName = [string]$entry.serviceName
    $sddl = [string]$entry.sddl

    if (-not (Get-ServiceExists -ServiceName $serviceName)) {
      Write-Host "[SKIP] サービスなし: $serviceName"
      continue
    }

    Set-ServiceSddl -ServiceName $serviceName -Sddl $sddl
    Write-Host "[OK] 復元: $serviceName"
  }
}

Assert-Administrator
$sid = Resolve-UserSid -AccountName $User

if ($Mode -eq "grant") {
  Grant-ServiceControl `
    -AccountName $User `
    -AccountSid $sid `
    -TargetServices $Services `
    -Path $BackupFile
}

if ($Mode -eq "backup") {
  Backup-ServiceSddl `
    -AccountName $User `
    -AccountSid $sid `
    -TargetServices $Services `
    -Path $BackupFile
}

if ($Mode -eq "restore") {
  Restore-ServiceSddl -Path $BackupFile
}

Write-Host "完了: mode=$Mode"
