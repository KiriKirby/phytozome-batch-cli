# The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
# you may not use this file except in compliance with the License. You may obtain a copy of the License at
# https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
# basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
# Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
# wangsychn. All Rights Reserved. Contributor(s): .

function Invoke-WebRequestWithRetry {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri,

        [Parameter(Mandatory = $true)]
        [string]$OutFile,

        [int]$MaxAttempts = 8,
        [int]$InitialDelaySeconds = 3
    )

    $targetDir = Split-Path -Parent $OutFile
    if (-not [string]::IsNullOrWhiteSpace($targetDir)) {
        New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
    }
    $tempFile = "$OutFile.download"
    $attempt = 0
    $delay = [Math]::Max(1, $InitialDelaySeconds)
    while ($true) {
        $attempt++
        try {
            Remove-Item -LiteralPath $tempFile -Force -ErrorAction SilentlyContinue
            Invoke-WebRequest -Uri $Uri -OutFile $tempFile
            Move-Item -LiteralPath $tempFile -Destination $OutFile -Force
            return
        } catch {
            Remove-Item -LiteralPath $tempFile -Force -ErrorAction SilentlyContinue
            Remove-Item -LiteralPath $OutFile -Force -ErrorAction SilentlyContinue
            if ($attempt -ge $MaxAttempts) {
                throw
            }
            Start-Sleep -Seconds $delay
            $delay = [Math]::Min($delay * 2, 30)
        }
    }
}
