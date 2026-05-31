# The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
# you may not use this file except in compliance with the License. You may obtain a copy of the License at
# https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
# basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
# Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
# wangsychn. All Rights Reserved. Contributor(s): .

function Get-MegaPHGORuntimeReleaseManifest {
    param(
        [string]$RepoRoot
    )

    if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
        $RepoRoot = Split-Path -Parent $PSScriptRoot
    }
    $manifestPath = Join-Path $RepoRoot "internal\megaphgo\runtime-release.json"
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw "Missing mega-phgo-runtime release manifest: $manifestPath"
    }
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($manifest.release_tag)) {
        throw "mega-phgo-runtime release manifest is missing release_tag: $manifestPath"
    }
    if ($null -eq $manifest.assets) {
        throw "mega-phgo-runtime release manifest is missing assets: $manifestPath"
    }
    return $manifest
}

function Get-MegaPHGORuntimeAssetName {
    param(
        [string]$Platform,
        [string]$RepoRoot
    )

    $manifest = Get-MegaPHGORuntimeReleaseManifest -RepoRoot $RepoRoot
    $assetName = $manifest.assets.$Platform
    if ([string]::IsNullOrWhiteSpace($assetName)) {
        throw "mega-phgo-runtime release manifest is missing asset name for platform '$Platform'"
    }
    return [string]$assetName
}

function Get-MegaPHGORuntimeReleaseTag {
    param(
        [string]$RepoRoot
    )

    $manifest = Get-MegaPHGORuntimeReleaseManifest -RepoRoot $RepoRoot
    return [string]$manifest.release_tag
}
