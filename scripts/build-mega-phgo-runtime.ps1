# The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
# you may not use this file except in compliance with the License. You may obtain a copy of the License at
# https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
# basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
# Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
# wangsychn. All Rights Reserved. Contributor(s): .

param(
    [string]$MegaSourceRoot = "",
    [string]$LazBuildPath = "",
    [string]$Platform = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
if ([string]::IsNullOrWhiteSpace($MegaSourceRoot)) {
    $MegaSourceRoot = Join-Path $repoRoot "_mega_source\MEGA12.1-source"
}
$MegaSourceRoot = (Resolve-Path -LiteralPath $MegaSourceRoot).Path
if ([string]::IsNullOrWhiteSpace($Platform)) {
    if ($IsWindows -or $env:OS -eq "Windows_NT") {
        $Platform = "windows-amd64"
    } elseif ($IsMacOS) {
        $Platform = "macos-amd64"
    } else {
        $Platform = "linux-amd64"
    }
}

function Resolve-HostPlatform {
    if ($IsWindows -or $env:OS -eq "Windows_NT") {
        return "windows-amd64"
    }
    if ($IsMacOS) {
        return "macos-amd64"
    }
    return "linux-amd64"
}

function RuntimeOwnedMuscleName {
    param([string]$PlatformName)
    switch ($PlatformName) {
        "windows-amd64" { return "muscleWin64.bin" }
        "linux-amd64" { return "muscleUnix64.exe" }
        "macos-amd64" { return "muscledarwin64" }
        default { throw "Unsupported mega-phgo-runtime platform: $PlatformName" }
    }
}

if ($Platform -ne (Resolve-HostPlatform)) {
    throw "Building mega-phgo-runtime for $Platform from this host is not configured. Build it on the matching platform or install/configure the Lazarus/FPC cross toolchain first."
}

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

function Resolve-LazBuild {
    param([string]$ExplicitPath)

    if (-not [string]::IsNullOrWhiteSpace($ExplicitPath)) {
        if (Test-Path -LiteralPath $ExplicitPath -PathType Leaf) {
            return (Resolve-Path -LiteralPath $ExplicitPath).Path
        }
        throw "lazbuild was not found: $ExplicitPath"
    }
    $command = Get-Command lazbuild -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($command) {
        return $command.Source
    }
    foreach ($candidate in @(
        "C:\lazarus\lazbuild.exe",
        "C:\Program Files\Lazarus\lazbuild.exe",
        "C:\Program Files (x86)\Lazarus\lazbuild.exe"
    )) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return $candidate
        }
    }
    throw "lazbuild is required to build mega-phgo-runtime. Install Lazarus/FPC, then rerun this script."
}

$project = Join-Path $MegaSourceRoot "PHgoRuntime\mega-phgo-runtime.lpi"
if (-not (Test-Path -LiteralPath $project -PathType Leaf)) {
    throw "Missing PHgo runtime Lazarus project: $project"
}

$lazbuild = Resolve-LazBuild $LazBuildPath
$runtimeDir = Join-Path $repoRoot ("assets\mega-phgo-runtime\" + $Platform + "\runtime")
$builtRuntimeExe = if ($Platform -eq "windows-amd64") { "mega-phgo-runtime.exe" } else { "mega-phgo-runtime" }
$runtimeExe = if ($Platform -eq "windows-amd64") { "mega-phgo-runtime.bin" } else { "mega-phgo-runtime" }
$targetPath = Join-Path $runtimeDir $runtimeExe

New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null

Push-Location (Split-Path -Parent $project)
try {
    & $lazbuild $project --build-mode=Release
    if ($LASTEXITCODE -ne 0) {
        throw "lazbuild failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}

$projectDir = Split-Path -Parent $project
$builtExe = Join-Path $projectDir $builtRuntimeExe
if (-not (Test-Path -LiteralPath $builtExe -PathType Leaf)) {
    $builtExe = Get-ChildItem -LiteralPath (Join-Path $projectDir "lib") -Recurse -File -Filter $builtRuntimeExe -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1 -ExpandProperty FullName
}
if ([string]::IsNullOrWhiteSpace($builtExe) -or -not (Test-Path -LiteralPath $builtExe -PathType Leaf)) {
    throw "Build completed but runtime executable was not found under $projectDir"
}

Copy-Item -LiteralPath $builtExe -Destination $targetPath -Force

$musclePath = Join-Path $runtimeDir (RuntimeOwnedMuscleName $Platform)
if (-not (Test-Path -LiteralPath $musclePath -PathType Leaf)) {
    throw "Missing runtime-owned MUSCLE executable in $runtimeDir. Restore the platform MUSCLE binary before packaging: $musclePath"
}
$probe = Invoke-RuntimeProbe -Executable $targetPath -MuscleExecutable $musclePath
if ($probe.ExitCode -ne 0 -or $probe.Output -notmatch "mega-phgo-runtime") {
    throw "Built runtime did not pass PHgo probe: $targetPath"
}

Write-Host "Built mega-phgo-runtime: $targetPath"
