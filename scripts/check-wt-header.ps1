param(
    [string]$SiteRoot = (Join-Path $PSScriptRoot '..\\docs')
)

$ErrorActionPreference = 'Stop'

function Get-RequiredSingleMatch {
    param(
        [string]$Html,
        [string]$Pattern,
        [string]$Description
    )

    $matches = [regex]::Matches($Html, $Pattern, [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
    if ($matches.Count -ne 1) {
        throw "$Description must appear exactly once; found $($matches.Count)."
    }
}

$indexPath = Join-Path $SiteRoot 'index.html'
$toolPath = Join-Path $SiteRoot 'wt.html'

foreach ($path in @($indexPath, $toolPath)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Required page was not found: $path"
    }
}

$indexHtml = [System.IO.File]::ReadAllText($indexPath)
$toolHtml = [System.IO.File]::ReadAllText($toolPath)

if ($toolHtml -match '(?is)<!doctype') {
    throw 'docs/wt.html must not declare a doctype. The legacy header requires BackCompat rendering.'
}

Get-RequiredSingleMatch $toolHtml '<table\s+border="0"\s+width="760"\s+cellspacing="0"\s+cellpadding="0"\s+bgcolor="#FFFFFF">' 'The locked 760-pixel outer page table'
Get-RequiredSingleMatch $toolHtml '<td\s+align="left"\s+valign="top"\s+height="160"\s+background="images/0\.jpg"\s+style="border-bottom:\s*10px\s+solid\s+#FFC000;\s*">' 'The locked 160-pixel banner row'
Get-RequiredSingleMatch $toolHtml '<table\s+border="0"\s+width="100%"\s+height="150"\s+cellspacing="0"\s+cellpadding="0">' 'The locked 150-pixel banner table'
Get-RequiredSingleMatch $toolHtml '<td\s+align="left"\s+width="100%"\s+height="50"\s+bgcolor="#D9F2D0">' 'The locked 50-pixel navigation row'

foreach ($marker in @('webbot', 'activex', 'fileupload', 'saveresults', '_derived')) {
    if ($toolHtml -match [regex]::Escape($marker)) {
        throw "docs/wt.html contains prohibited legacy component marker: $marker"
    }
}

if ($toolHtml -notmatch '(?is)<nobr>TOOLS</nobr>') {
    throw 'The TOOLS item must remain the selected navigation item.'
}

foreach ($pattern in @(
    '<table\s+border="0"\s+width="760"\s+cellspacing="0"\s+cellpadding="0"\s+bgcolor="#FFFFFF">',
    '<td\s+align="left"\s+valign="top"\s+height="160"\s+background="images/0\.jpg"\s+style="border-bottom:\s*10px\s+solid\s+#FFC000;\s*">',
    '<table\s+border="0"\s+width="100%"\s+height="150"\s+cellspacing="0"\s+cellpadding="0">',
    '<td\s+align="left"\s+width="100%"\s+height="50"\s+bgcolor="#D9F2D0">'
)) {
    if ($indexHtml -notmatch $pattern) {
        throw "docs/index.html no longer provides the expected legacy header anchor: $pattern"
    }
}

Write-Host 'wt.html header contract passed: BackCompat source shape and locked legacy geometry are intact.'
