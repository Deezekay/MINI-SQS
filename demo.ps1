# Mini-SQS Demo Script
# Ensure the server is running in a separate terminal: go run cmd/server/main.go

$BaseUrl = "http://localhost:8080"

Write-Host "1. Enqueueing Task..." -ForegroundColor Cyan
$Enqueue = Invoke-RestMethod -Method Post -Uri "$BaseUrl/enqueue" -Body '{"task_id": "demo-task-1", "payload": "Hello Mini-SQS"}' -ContentType "application/json"
Write-Host "Enqueued (200 OK)"

Write-Host "`n2. Polling for Task..." -ForegroundColor Cyan
$Task = Invoke-RestMethod -Method Post -Uri "$BaseUrl/poll" -Body '{"worker_id": "worker-1"}' -ContentType "application/json"
Write-Host "Received Task:"
$Task | Format-List

if ($Task) {
    Write-Host "`n3. Acknowledging Task..." -ForegroundColor Cyan
    $AckBody = @{
        task_id = $Task.id
        worker_id = "worker-1"
    } | ConvertTo-Json
    Invoke-RestMethod -Method Post -Uri "$BaseUrl/ack" -Body $AckBody -ContentType "application/json"
    Write-Host "Acknowledged"
}

Write-Host "`n4. Checking Metrics..." -ForegroundColor Cyan
$Metrics = Invoke-RestMethod -Method Get -Uri "$BaseUrl/metrics"
$Metrics | Format-List
