# The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
# you may not use this file except in compliance with the License. You may obtain a copy of the License at
# https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
# basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
# Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
# wangsychn. All Rights Reserved. Contributor(s): .

param(
    [string]$SmallSource = "docs\logo3small.png",
    [string]$LargeSource = "docs\logo3large.png"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$smallSourcePath = Join-Path $repoRoot $SmallSource
$largeSourcePath = Join-Path $repoRoot $LargeSource
$launcherDir = Join-Path $repoRoot "cmd\phytozome-go-winlauncher"
$iconPath = Join-Path $launcherDir "phytozome-go.ico"
$sysoPath = Join-Path $launcherDir "rsrc_windows_amd64.syso"
$toolBinDir = Join-Path $repoRoot "bin\tooling\gobin"

if (-not (Test-Path -LiteralPath $smallSourcePath -PathType Leaf)) {
    throw "Small icon source not found: $smallSourcePath"
}
if (-not (Test-Path -LiteralPath $largeSourcePath -PathType Leaf)) {
    throw "Large icon source not found: $largeSourcePath"
}

Add-Type -AssemblyName System.Drawing

function Resolve-RsrcCommand {
    $command = Get-Command "rsrc" -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    New-Item -ItemType Directory -Force -Path $toolBinDir | Out-Null

    $existingGobin = [Environment]::GetEnvironmentVariable("GOBIN", "Process")
    $existingPath = [Environment]::GetEnvironmentVariable("PATH", "Process")
    try {
        [Environment]::SetEnvironmentVariable("GOBIN", $toolBinDir, "Process")
        if (-not (($existingPath -split ';') -contains $toolBinDir)) {
            [Environment]::SetEnvironmentVariable("PATH", "$toolBinDir;$existingPath", "Process")
        }
        & go install github.com/akavel/rsrc@latest
        if ($LASTEXITCODE -ne 0) {
            throw "go install github.com/akavel/rsrc@latest failed with exit code $LASTEXITCODE"
        }
    } finally {
        [Environment]::SetEnvironmentVariable("GOBIN", $existingGobin, "Process")
        [Environment]::SetEnvironmentVariable("PATH", $existingPath, "Process")
    }

    $installed = Join-Path $toolBinDir "rsrc.exe"
    if (-not (Test-Path -LiteralPath $installed -PathType Leaf)) {
        throw "rsrc was installed but not found at $installed"
    }
    return $installed
}

function New-IconPngBytes {
    param(
        [System.Drawing.Image]$SourceImage,
        [int]$Size
    )

    $bitmap = New-Object System.Drawing.Bitmap $Size, $Size, ([System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    try {
        $graphics.Clear([System.Drawing.Color]::Transparent)
        $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
        $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
        $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
        $graphics.DrawImage($SourceImage, 0, 0, $Size, $Size)

        $stream = New-Object System.IO.MemoryStream
        try {
            $bitmap.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
            return $stream.ToArray()
        } finally {
            $stream.Dispose()
        }
    } finally {
        $graphics.Dispose()
        $bitmap.Dispose()
    }
}

function Write-UInt16LE {
    param(
        [System.IO.BinaryWriter]$Writer,
        [int]$Value
    )
    $Writer.Write([uint16]$Value)
}

function Write-UInt32LE {
    param(
        [System.IO.BinaryWriter]$Writer,
        [int]$Value
    )
    $Writer.Write([uint32]$Value)
}

$smallSizes = @(16, 20, 24, 32, 40)
$sizes = @(16, 20, 24, 32, 40, 48, 64, 128, 256)
$smallImage = [System.Drawing.Image]::FromFile($smallSourcePath)
$largeImage = [System.Drawing.Image]::FromFile($largeSourcePath)
try {
    $entries = foreach ($size in $sizes) {
        $sourceImage = if ($smallSizes -contains $size) { $smallImage } else { $largeImage }
        [pscustomobject]@{
            Size = $size
            Bytes = New-IconPngBytes -SourceImage $sourceImage -Size $size
        }
    }
} finally {
    $smallImage.Dispose()
    $largeImage.Dispose()
}

$iconStream = New-Object System.IO.MemoryStream
$writer = New-Object System.IO.BinaryWriter $iconStream
try {
    Write-UInt16LE $writer 0
    Write-UInt16LE $writer 1
    Write-UInt16LE $writer $entries.Count

    $offset = 6 + (16 * $entries.Count)
    foreach ($entry in $entries) {
        $dimension = if ($entry.Size -eq 256) { 0 } else { $entry.Size }
        $writer.Write([byte]$dimension)
        $writer.Write([byte]$dimension)
        $writer.Write([byte]0)
        $writer.Write([byte]0)
        Write-UInt16LE $writer 1
        Write-UInt16LE $writer 32
        Write-UInt32LE $writer $entry.Bytes.Length
        Write-UInt32LE $writer $offset
        $offset += $entry.Bytes.Length
    }

    foreach ($entry in $entries) {
        $writer.Write([byte[]]$entry.Bytes)
    }
    [System.IO.File]::WriteAllBytes($iconPath, $iconStream.ToArray())
} finally {
    $writer.Dispose()
    $iconStream.Dispose()
}

$rsrcPath = Resolve-RsrcCommand
& $rsrcPath -arch amd64 -ico $iconPath -o $sysoPath

Write-Host "Icon written to: $iconPath"
Write-Host "Resource written to: $sysoPath"
