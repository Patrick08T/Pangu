$projectRoot = Split-Path $PSScriptRoot -Parent
$connectionPath = Join-Path $projectRoot '.tools/hatchet-connection.json'
if (!(Test-Path -LiteralPath $connectionPath)) { throw '请先启动本地 Hatchet，等待连接文件生成。' }
$connection = Get-Content -LiteralPath $connectionPath -Raw -Encoding UTF8 | ConvertFrom-Json
$env:HATCHET_CLIENT_TOKEN = $connection.token
$env:HATCHET_CLIENT_TENANT_ID = $connection.tenant_id
$env:HATCHET_CLIENT_HOST_PORT = $connection.grpc_address
$env:HATCHET_CLIENT_SERVER_URL = $connection.api_url
$env:HATCHET_CLIENT_TLS_STRATEGY = 'none'
$env:HATCHET_CLIENT_LOG_LEVEL = 'warn'
