$ErrorActionPreference = "Stop"

Set-Location (Join-Path $PSScriptRoot "..")

$migrations = Get-ChildItem "store/migrations/*.sql" | Sort-Object Name

foreach ($file in $migrations) {
    $version = $file.Name
    $prevPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $check = docker compose exec -T postgres psql -U atlas -d atlas -tAc `
        "SELECT 1 FROM schema_migrations WHERE version='$version'" 2>&1
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $prevPreference
    $applied = ($exitCode -eq 0) -and ("$check" -match "1")

    if ($applied) {
        Write-Host "Skipping $($file.Name) (already applied)"
        continue
    }

    Write-Host "Applying $($file.Name)..."
    Get-Content $file.FullName -Raw | docker compose exec -T postgres psql -U atlas -d atlas -v ON_ERROR_STOP=1
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    docker compose exec -T postgres psql -U atlas -d atlas `
        -c "INSERT INTO schema_migrations (version) VALUES ('$version')"
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

Write-Host "Migrations complete."
