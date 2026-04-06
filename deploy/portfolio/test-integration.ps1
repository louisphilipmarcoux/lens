# Portfolio Integration Test Script
# Tests all three projects working together.
#
# Prerequisites: docker compose up from deploy/portfolio/
#
# Usage: .\deploy\portfolio\test-integration.ps1

$ErrorActionPreference = "Stop"

Write-Host "=== Portfolio Integration Test ===" -ForegroundColor Cyan
Write-Host ""

# Wait for services
Write-Host "[1/6] Waiting for services..." -ForegroundColor Yellow
$maxRetries = 30
for ($i = 0; $i -lt $maxRetries; $i++) {
    try {
        $null = Invoke-RestMethod -Uri "http://localhost:8080/health" -TimeoutSec 2
        break
    } catch {
        if ($i -eq $maxRetries - 1) { Write-Error "Ingest service not ready"; exit 1 }
        Start-Sleep 2
    }
}
Write-Host "  Ingest service: OK" -ForegroundColor Green

try {
    $null = Invoke-RestMethod -Uri "http://localhost:8081/health" -TimeoutSec 2
    Write-Host "  Query service:  OK" -ForegroundColor Green
} catch {
    Write-Host "  Query service:  UNAVAILABLE" -ForegroundColor Red
}

# Check Raft cluster health
Write-Host ""
Write-Host "[2/6] Checking Raft KV Store cluster..." -ForegroundColor Yellow
foreach ($port in @(9191, 9192, 9193)) {
    try {
        $health = Invoke-RestMethod -Uri "http://localhost:$port/health" -TimeoutSec 2
        Write-Host "  Raft node (port $port): OK" -ForegroundColor Green
    } catch {
        Write-Host "  Raft node (port $port): UNAVAILABLE" -ForegroundColor Red
    }
}

# Scrape Raft Prometheus metrics
Write-Host ""
Write-Host "[3/6] Scraping Raft Prometheus metrics..." -ForegroundColor Yellow
try {
    $metrics = Invoke-WebRequest -Uri "http://localhost:9191/metrics" -TimeoutSec 5
    $lineCount = ($metrics.Content -split "`n").Count
    Write-Host "  Raft node 1: $lineCount metric lines" -ForegroundColor Green
} catch {
    Write-Host "  Could not scrape Raft metrics" -ForegroundColor Red
}

# Ingest test data
Write-Host ""
Write-Host "[4/6] Ingesting test data (metrics + logs + traces)..." -ForegroundColor Yellow
$ts = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
$body = @{
    metrics = @(
        @{name="cpu.user_percent"; tags=@{host="portfolio-test"; project="lens"}; value=42.5; timestamp=$ts; type=0},
        @{name="raft.elections_total"; tags=@{host="raft-node-1"; project="raft"}; value=3; timestamp=$ts; type=1},
        @{name="container.cpu_usage_percent"; tags=@{host="vessel"; container="raft-1"; project="vessel"}; value=12.5; timestamp=$ts; type=0}
    )
    logs = @(
        @{timestamp=$ts; service="lens-agent"; host="portfolio-test"; level="info"; message="collecting metrics from all portfolio projects"},
        @{timestamp=$ts; service="raft-node-1"; host="raft-1"; level="info"; message="leader elected for term 5"},
        @{timestamp=$ts; service="vessel"; host="vessel"; level="info"; message="container raft-1 started with PID 12345"}
    )
    spans = @(
        @{trace_id="portfolio-trace-001"; span_id="root"; service="lens-query"; operation="GET /api/v1/query_range"; start_time=$ts; duration=50000000; status=1; tags=@{"http.method"="GET"}},
        @{trace_id="portfolio-trace-001"; span_id="ch-query"; parent_id="root"; service="clickhouse"; operation="SELECT metrics"; start_time=$ts; duration=25000000; status=1},
        @{trace_id="portfolio-trace-001"; span_id="raft-read"; parent_id="root"; service="raft-kv"; operation="linearizable_read"; start_time=$ts; duration=8000000; status=1; tags=@{key="/lens/dashboards/main"}}
    )
} | ConvertTo-Json -Depth 4

$response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/ingest" -Method Post -ContentType "application/json" -Body $body
Write-Host "  Ingest response: $($response.status)" -ForegroundColor Green

# Wait for flush
Write-Host ""
Write-Host "[5/6] Waiting for ClickHouse flush (6s)..." -ForegroundColor Yellow
Start-Sleep 6

# Query data back
Write-Host ""
Write-Host "[6/6] Querying data back..." -ForegroundColor Yellow

$startTime = (Get-Date).AddHours(-1).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$endTime = (Get-Date).AddHours(1).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

# Metrics
try {
    $metrics = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/query_range?query=cpu.user_percent&start=$startTime&end=$endTime&step=1m"
    Write-Host "  Metrics query:  $($metrics.Count) series returned" -ForegroundColor Green
} catch {
    Write-Host "  Metrics query:  FAILED" -ForegroundColor Red
}

# Logs
try {
    $logs = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/logs?start=$startTime&end=$endTime"
    Write-Host "  Logs query:     $($logs.total) records" -ForegroundColor Green
} catch {
    Write-Host "  Logs query:     FAILED" -ForegroundColor Red
}

# Traces
try {
    $trace = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/traces/portfolio-trace-001"
    Write-Host "  Trace lookup:   $($trace.spans.Count) spans, services: $($trace.services -join ', ')" -ForegroundColor Green
} catch {
    Write-Host "  Trace lookup:   FAILED" -ForegroundColor Red
}

Write-Host ""
Write-Host "=== Integration Test Complete ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Dashboard: http://localhost:3000" -ForegroundColor White
Write-Host "Ingest:    http://localhost:8080/health" -ForegroundColor White
Write-Host "Query:     http://localhost:8081/health" -ForegroundColor White
Write-Host "Raft:      http://localhost:9191/metrics" -ForegroundColor White
