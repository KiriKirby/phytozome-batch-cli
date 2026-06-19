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

function Invoke-RuntimeProbe {
    param(
        [string]$Executable,
        [string]$MuscleExecutable = ""
    )

    $probeExecutable = $Executable
    $tempDir = $null
    if ($IsWindows -or $env:OS -eq "Windows_NT") {
        if ([IO.Path]::GetExtension($Executable).ToLowerInvariant() -eq ".bin") {
            $tempDir = Join-Path ([IO.Path]::GetTempPath()) ("phytozome-go-megaphgo-probe-" + [guid]::NewGuid().ToString("N"))
            New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
            $probeExecutable = Join-Path $tempDir "mega-phgo-runtime.exe"
            Copy-Item -LiteralPath $Executable -Destination $probeExecutable -Force
            if (-not [string]::IsNullOrWhiteSpace($MuscleExecutable)) {
                Copy-Item -LiteralPath $MuscleExecutable -Destination (Join-Path $tempDir (Split-Path -Leaf $MuscleExecutable)) -Force
            }
        }
    }
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $probeExecutable
    $psi.Arguments = "--phgo-runtime-probe"
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true

    try {
        $proc = New-Object System.Diagnostics.Process
        $proc.StartInfo = $psi
        $proc.Start() | Out-Null
        $stdout = $proc.StandardOutput.ReadToEnd()
        $stderr = $proc.StandardError.ReadToEnd()
        $proc.WaitForExit()

        return [pscustomobject]@{
            ExitCode = $proc.ExitCode
            Output = (($stdout + "`n" + $stderr).Trim())
        }
    } finally {
        if ($tempDir -and (Test-Path -LiteralPath $tempDir)) {
            Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

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
    $muscleExe = Join-Path $RuntimePath $MuscleExecutable
    if (-not (Test-Path -LiteralPath $muscleExe -PathType Leaf)) {
        throw "Missing runtime-owned MUSCLE executable: $muscleExe"
    }
    $probe = Invoke-RuntimeProbe -Executable $runtimeExe -MuscleExecutable $muscleExe
    if ($probe.ExitCode -ne 0 -or $probe.Output -notmatch "mega-phgo-runtime") {
        throw "Invalid mega-phgo-runtime executable. The file must be the PHgo custom runtime and respond to --phgo-runtime-probe: $runtimeExe"
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

Assert-Runtime -RuntimePath $windowsRuntime -Executable "mega-phgo-runtime.bin" -MuscleExecutable "muscleWin64.bin"
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
