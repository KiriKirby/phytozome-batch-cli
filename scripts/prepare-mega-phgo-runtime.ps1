# The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
# you may not use this file except in compliance with the License. You may obtain a copy of the License at
# https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
# basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
# Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
# wangsychn. All Rights Reserved. Contributor(s): .

param()

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
$runtimeRoot = Join-Path $repoRoot "assets\mega-phgo-runtime"

function Assert-Runtime {
    param(
        [string]$RuntimePath,
        [string]$Executable,
        [string]$MuscleExecutable
    )

    if (-not (Test-Path -LiteralPath $RuntimePath -PathType Container)) {
        throw "Missing pre-extracted mega-phgo-runtime directory: $RuntimePath"
    }
    $runtimeExe = Join-Path $RuntimePath $Executable
    if (-not (Test-Path -LiteralPath $runtimeExe -PathType Leaf)) {
        throw "Missing pre-extracted mega-phgo-runtime executable: $runtimeExe"
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

function Test-RuntimeExecutable {
    param(
        [string]$RuntimePath,
        [string]$Executable
    )

    return (Test-Path -LiteralPath (Join-Path $RuntimePath $Executable) -PathType Leaf)
}

$windowsRuntime = Join-Path $runtimeRoot "windows-amd64\runtime"
$linuxRuntime = Join-Path $runtimeRoot "linux-amd64\runtime"
$macRuntime = Join-Path $runtimeRoot "macos-amd64\runtime"

Assert-Runtime -RuntimePath $windowsRuntime -Executable "mega-phgo-runtime.exe" -MuscleExecutable "muscleWin64.exe"
if (Test-RuntimeExecutable -RuntimePath $linuxRuntime -Executable "mega-phgo-runtime") {
    Assert-Runtime -RuntimePath $linuxRuntime -Executable "mega-phgo-runtime" -MuscleExecutable "muscleUnix64.exe"
} else {
    Write-Host "Skipping linux-amd64 mega-phgo-runtime validation because no built runtime is present."
}
if (Test-RuntimeExecutable -RuntimePath $macRuntime -Executable "mega-phgo-runtime") {
    Assert-Runtime -RuntimePath $macRuntime -Executable "mega-phgo-runtime" -MuscleExecutable "muscledarwin64"
} else {
    Write-Host "Skipping macos-amd64 mega-phgo-runtime validation because no built runtime is present."
}

Write-Host "mega-phgo-runtime directories are ready under assets\mega-phgo-runtime."
