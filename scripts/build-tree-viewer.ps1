# The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
# you may not use this file except in compliance with the License. You may obtain a copy of the License at
# https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
# basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
# Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
# wangsychn. All Rights Reserved. Contributor(s): .

param()

$ErrorActionPreference = "Stop"

function Resolve-NpmCommand {
    $commands = @(
        (Get-Command npm.cmd -ErrorAction SilentlyContinue),
        (Get-Command npm -ErrorAction SilentlyContinue)
    ) | Where-Object { $null -ne $_ }
    if ($commands.Count -gt 0) {
        return $commands[0].Source
    }

    $candidates = @(
        (Join-Path ${env:ProgramFiles} "nodejs\npm.cmd"),
        (Join-Path ${env:ProgramFiles} "nodejs\node_modules\npm\bin\npm.cmd"),
        (Join-Path ${env:LOCALAPPDATA} "Programs\nodejs\npm.cmd")
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }

    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return $candidate
        }
    }

    throw "npm was not found. Install Node.js/npm or add npm.cmd to PATH before building the tree viewer."
}

function Invoke-Npm {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Arguments
    )

    if (-not $script:NpmCommand) {
        $script:NpmCommand = Resolve-NpmCommand
    }

    $npmDir = Split-Path -Parent $script:NpmCommand
    $originalPath = $env:Path
    if (-not [string]::IsNullOrWhiteSpace($npmDir)) {
        $env:Path = "$npmDir;$originalPath"
    }

    try {
        & $script:NpmCommand @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "npm $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
        }
    } finally {
        $env:Path = $originalPath
    }
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
$viewerRoot = Join-Path $repoRoot "tree-viewer"
$targetRoot = Join-Path $repoRoot "internal\phylo\viewer_assets"
$targetAssetsRoot = Join-Path $targetRoot "assets"
$skipTreeViewerBuild = $false
if ($env:PHGO_SKIP_TREE_VIEWER_BUILD -match '^(?i:1|true|yes|y)$') {
    $skipTreeViewerBuild = $true
}
if (-not $skipTreeViewerBuild) {
    $codexNode = Get-Command node -ErrorAction SilentlyContinue
    if ($codexNode -and $codexNode.Source -like '*OpenAI.Codex*' -and (Test-Path -LiteralPath $targetRoot -PathType Container) -and (Test-Path -LiteralPath (Join-Path $targetRoot "index.html") -PathType Leaf)) {
        Write-Host "Detected Codex-managed Node environment; reusing committed viewer assets to avoid npm/reactree patch incompatibilities."
        $skipTreeViewerBuild = $true
    }
}

if (-not (Test-Path -LiteralPath (Join-Path $viewerRoot "package.json") -PathType Leaf)) {
    throw "Missing tree viewer package.json: $viewerRoot"
}

if (-not $skipTreeViewerBuild) {
    Push-Location $viewerRoot
    try {
        Invoke-Npm @("ci")
        Invoke-Npm @("test")
        $distRoot = Join-Path $viewerRoot "dist"
        if (Test-Path -LiteralPath $distRoot) {
            Get-ChildItem -LiteralPath $distRoot -Recurse -Force | Remove-Item -Force -Recurse
        }
        Invoke-Npm @("run", "build")
    } finally {
        Pop-Location
    }

    if (-not (Test-Path -LiteralPath $targetRoot)) {
        New-Item -ItemType Directory -Force -Path $targetRoot | Out-Null
    }
    if (-not (Test-Path -LiteralPath $targetAssetsRoot)) {
        New-Item -ItemType Directory -Force -Path $targetAssetsRoot | Out-Null
    }

    foreach ($name in @("index.html", "phgo-icon.png")) {
        $path = Join-Path $targetRoot $name
        if (Test-Path -LiteralPath $path) {
            Remove-Item -LiteralPath $path -Force
        }
    }
    if (Test-Path -LiteralPath $targetAssetsRoot) {
        $persistentAssetDirs = @("jalviewjs", "msaexpor")
        Get-ChildItem -LiteralPath $targetAssetsRoot -Force |
            Where-Object { $persistentAssetDirs -notcontains $_.Name } |
            Remove-Item -Recurse -Force
    }

    Copy-Item -LiteralPath (Join-Path $viewerRoot "dist\index.html") -Destination $targetRoot -Force
    if (Test-Path -LiteralPath (Join-Path $viewerRoot "dist\phgo-icon.png")) {
        Copy-Item -LiteralPath (Join-Path $viewerRoot "dist\phgo-icon.png") -Destination $targetRoot -Force
    }
    New-Item -ItemType Directory -Force -Path $targetAssetsRoot | Out-Null
    Get-ChildItem -LiteralPath (Join-Path $viewerRoot "dist\assets") -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $targetAssetsRoot -Recurse -Force
    }
    Write-Host "Reactree viewer assets copied to internal\phylo\viewer_assets."
    exit 0
}

if (-not (Test-Path -LiteralPath $targetRoot -PathType Container)) {
    throw "Cannot skip tree viewer build because viewer assets are missing: $targetRoot"
}
if (-not (Test-Path -LiteralPath (Join-Path $targetRoot "index.html") -PathType Leaf)) {
    throw "Cannot skip tree viewer build because viewer assets are incomplete: $targetRoot"
}
Write-Host "Skipping tree viewer rebuild and reusing committed viewer assets from internal\phylo\viewer_assets."
