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
$viewerRoot = Join-Path $repoRoot "tree-viewer"
$targetRoot = Join-Path $repoRoot "internal\phylo\viewer_assets"

if (-not (Test-Path -LiteralPath (Join-Path $viewerRoot "package.json") -PathType Leaf)) {
    throw "Missing tree viewer package.json: $viewerRoot"
}

Push-Location $viewerRoot
try {
    npm install
    npm test
    $distRoot = Join-Path $viewerRoot "dist"
    if (Test-Path -LiteralPath $distRoot) {
        Get-ChildItem -LiteralPath $distRoot -Recurse -Force | Remove-Item -Force -Recurse
    }
    npm run build
} finally {
    Pop-Location
}

if (Test-Path -LiteralPath $targetRoot) {
    Get-ChildItem -LiteralPath $targetRoot -Recurse -Force | Remove-Item -Force -Recurse
} else {
    New-Item -ItemType Directory -Force -Path $targetRoot | Out-Null
}

Copy-Item -Path (Join-Path $viewerRoot "dist\*") -Destination $targetRoot -Recurse -Force
Write-Host "Reactree viewer assets copied to internal\phylo\viewer_assets."
