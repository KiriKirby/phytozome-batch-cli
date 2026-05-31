# The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
# you may not use this file except in compliance with the License. You may obtain a copy of the License at
# https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
# basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
# Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
# wangsychn. All Rights Reserved. Contributor(s): .

param(
    [ValidateSet("all", "windows-amd64", "linux-amd64", "macos-amd64")]
    [string]$Platform = "all"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
. (Join-Path $scriptDir "mega-phgo-runtime-release.ps1")
$binDir = Join-Path $repoRoot "bin"
$runtimeRoot = Join-Path $repoRoot "assets\mega-phgo-runtime"
$windowsRuntime = Join-Path $runtimeRoot "windows-amd64\runtime"
$linuxRuntime = Join-Path $runtimeRoot "linux-amd64\runtime"
$macRuntime = Join-Path $runtimeRoot "macos-amd64\runtime"

New-Item -ItemType Directory -Force -Path $binDir | Out-Null

function Assert-Runtime {
    param(
        [string]$RuntimePath,
        [string]$Executable,
        [string]$MuscleExecutable
    )

    if (-not (Test-Path -LiteralPath $RuntimePath -PathType Container)) {
        throw "Missing mega-phgo-runtime directory: $RuntimePath"
    }
    $runtimeExe = Join-Path $RuntimePath $Executable
    if (-not (Test-Path -LiteralPath $runtimeExe -PathType Leaf)) {
        throw "Missing mega-phgo-runtime executable: $runtimeExe"
    }
    $probeOutput = & $runtimeExe --phgo-runtime-probe 2>&1
    if ($LASTEXITCODE -ne 0 -or (($probeOutput -join "`n") -notmatch "mega-phgo-runtime")) {
        throw "Invalid mega-phgo-runtime executable. The file must be the PHgo custom runtime and respond to --phgo-runtime-probe: $runtimeExe"
    }
    $muscleExe = Join-Path $RuntimePath $MuscleExecutable
    if (-not (Test-Path -LiteralPath $muscleExe -PathType Leaf)) {
        throw "Missing runtime-owned MUSCLE executable: $muscleExe"
    }
}

function Test-RuntimeReady {
    param(
        [string]$RuntimePath,
        [string]$Executable,
        [string]$MuscleExecutable
    )

    if (-not (Test-Path -LiteralPath $RuntimePath -PathType Container)) {
        return $false
    }
    if (-not (Test-Path -LiteralPath (Join-Path $RuntimePath $Executable) -PathType Leaf)) {
        return $false
    }
    if (-not (Test-Path -LiteralPath (Join-Path $RuntimePath $MuscleExecutable) -PathType Leaf)) {
        return $false
    }
    return $true
}

function Pack-Zip {
    param(
        [string]$Source,
        [string]$Destination
    )

    Remove-Item -LiteralPath $Destination -Force -ErrorAction SilentlyContinue
    Compress-Archive -Path (Join-Path $Source "*") -DestinationPath $Destination -Force
}

if ($Platform -eq "all" -or $Platform -eq "windows-amd64") {
    Assert-Runtime -RuntimePath $windowsRuntime -Executable "mega-phgo-runtime.exe" -MuscleExecutable "muscleWin64.exe"
    Pack-Zip $windowsRuntime (Join-Path $binDir (Get-MegaPHGORuntimeAssetName -Platform "windows-amd64" -RepoRoot $repoRoot))
}
if ($Platform -eq "all" -or $Platform -eq "linux-amd64") {
    if ($Platform -eq "linux-amd64" -or (Test-RuntimeReady -RuntimePath $linuxRuntime -Executable "mega-phgo-runtime" -MuscleExecutable "muscleUnix64.exe")) {
        Assert-Runtime -RuntimePath $linuxRuntime -Executable "mega-phgo-runtime" -MuscleExecutable "muscleUnix64.exe"
        Pack-Zip $linuxRuntime (Join-Path $binDir (Get-MegaPHGORuntimeAssetName -Platform "linux-amd64" -RepoRoot $repoRoot))
    } else {
        Write-Host "Skipping linux-amd64 mega-phgo-runtime packaging because no built runtime is present."
    }
}
if ($Platform -eq "all" -or $Platform -eq "macos-amd64") {
    if ($Platform -eq "macos-amd64" -or (Test-RuntimeReady -RuntimePath $macRuntime -Executable "mega-phgo-runtime" -MuscleExecutable "muscledarwin64")) {
        Assert-Runtime -RuntimePath $macRuntime -Executable "mega-phgo-runtime" -MuscleExecutable "muscledarwin64"
        Pack-Zip $macRuntime (Join-Path $binDir (Get-MegaPHGORuntimeAssetName -Platform "macos-amd64" -RepoRoot $repoRoot))
    } else {
        Write-Host "Skipping macos-amd64 mega-phgo-runtime packaging because no built runtime is present."
    }
}

Write-Host "mega-phgo-runtime archives written to: $binDir"
