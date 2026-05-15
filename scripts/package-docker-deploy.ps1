# Pack repo for Docker Compose deploy (includes Mopc-web-site + its Dockerfile; excludes deps/build output).
# Usage: pwsh ./scripts/package-docker-deploy.ps1
# Output: deploy-packages/afb-new-api-docker-deploy-<timestamp>.zip

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$outDir = Join-Path $repoRoot "deploy-packages"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$zipName = "afb-new-api-docker-deploy-$stamp.zip"
$zipPath = Join-Path $outDir $zipName

$tempRoot = Join-Path $env:TEMP ("afb-docker-staging-" + [Guid]::NewGuid().ToString())
$staging = Join-Path $tempRoot "afb-new-api"
try {
    New-Item -ItemType Directory -Force -Path $staging | Out-Null

    # /XD: exclude every directory with these names (any depth). Keeps Mopc-web-site sources & Docker files.
    # deploy-packages: never pack previous *.zip into the next zip (was causing runaway size)
    $excludeDirs = @(
        ".git", ".github", "node_modules", "dist", "upload", "data", "logs",
        "deploy-packages",
        "openclaw-afb",
        "backups", "docfile", "UIPic",
        "plans", ".idea", ".vscode", ".zed", ".history", ".cache",
        ".tmp-go-cache", ".gocache", ".gomodcache", ".gopath",
        ".eslintcache", ".claude", "tiktoken_cache", "build", "release", "electron\dist"
    )
    # Note: Go builds on Windows sometimes leave backup artifacts like "main.exe~" (not matched by "*.exe").
    # Exclude those to avoid bloating the deploy zip.
    $excludeFiles = @("*.exe", "*.exe~", "*.db", "*.db-journal")

    $rcArgs = @(
        $repoRoot, $staging,
        "/E", "/NFL", "/NDL", "/NJH", "/NJS", "/nc", "/ns", "/np",
        "/XD"
    ) + $excludeDirs + @("/XF") + $excludeFiles

    & robocopy @rcArgs | Out-Null
    $rc = $LASTEXITCODE
    if ($rc -ge 8) {
        throw "robocopy failed with exit code $rc"
    }

    Compress-Archive -Path (Join-Path $staging "*") -DestinationPath $zipPath -Force
    $sizeMb = [math]::Round((Get-Item $zipPath).Length / 1MB, 2)
    Write-Host "OK: $zipPath ($sizeMb MB)"
}
finally {
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
