param(
    [string]$ListenAddress = "127.0.0.1:52736",
    [string]$BridgePath = (Join-Path $PSScriptRoot "..\bin\cebridge-windows-arm64.exe"),
    [string]$AllowedRemoteAddress = ""
)

$ErrorActionPreference = "Stop"

if ($ListenAddress -notmatch "^(?<Host>.+):(?<Port>\d+)$") {
    throw "ListenAddress must use host:port format"
}
$listenHost = $Matches.Host
$listenPort = [int]$Matches.Port
$isLoopback = $listenHost -in @("127.0.0.1", "localhost", "::1")
if (-not $isLoopback -and [string]::IsNullOrWhiteSpace($AllowedRemoteAddress)) {
    throw "AllowedRemoteAddress is required when listening beyond loopback"
}

$resolvedBridgePath = (Resolve-Path $BridgePath).Path
$bridgeFileName = [IO.Path]::GetFileName($resolvedBridgePath)
$existingListeners = Get-NetTCPConnection -LocalPort $listenPort -State Listen -ErrorAction SilentlyContinue
foreach ($existingListener in $existingListeners) {
    $existingProcess = Get-CimInstance Win32_Process -Filter "ProcessId=$($existingListener.OwningProcess)"
    if ($existingProcess.Name -ne $bridgeFileName) {
        throw "port $listenPort is already owned by $($existingProcess.Name) (PID $($existingProcess.ProcessId))"
    }
    Stop-Process -Id $existingProcess.ProcessId -Force
}

if (-not $isLoopback) {
    $firewallRuleName = "cebridge $listenPort from $AllowedRemoteAddress"
    $firewallRule = Get-NetFirewallRule -DisplayName $firewallRuleName -ErrorAction SilentlyContinue
    if ($firewallRule) {
        $firewallRule | Set-NetFirewallRule -Enabled True -Direction Inbound -Action Allow -Profile Any
        $firewallRule | Get-NetFirewallAddressFilter | Set-NetFirewallAddressFilter -RemoteAddress $AllowedRemoteAddress
    } else {
        New-NetFirewallRule `
            -DisplayName $firewallRuleName `
            -Direction Inbound `
            -Action Allow `
            -Protocol TCP `
            -LocalPort $listenPort `
            -RemoteAddress $AllowedRemoteAddress `
            -Profile Any | Out-Null
    }
}

$stdoutPath = Join-Path $env:TEMP "cebridge.stdout.log"
$stderrPath = Join-Path $env:TEMP "cebridge.stderr.log"
$process = Start-Process `
    -FilePath $resolvedBridgePath `
    -ArgumentList "--listen=$ListenAddress" `
    -WindowStyle Hidden `
    -RedirectStandardOutput $stdoutPath `
    -RedirectStandardError $stderrPath `
    -PassThru

Start-Sleep -Seconds 2

$listener = Get-NetTCPConnection -LocalPort $listenPort -State Listen -ErrorAction SilentlyContinue |
    Where-Object { $_.OwningProcess -eq $process.Id } |
    Select-Object -First 1
if (-not $listener) {
    $standardError = if (Test-Path $stderrPath) { (Get-Content $stderrPath | Out-String).Trim() } else { "" }
    throw "cebridge did not start listening on $ListenAddress. $standardError"
}
[PSCustomObject]@{
    ProcessId = $process.Id
    CommandLine = (Get-CimInstance Win32_Process -Filter "ProcessId=$($process.Id)").CommandLine
    LocalAddress = $listener.LocalAddress
    LocalPort = $listener.LocalPort
    StandardOutput = if (Test-Path $stdoutPath) { (Get-Content $stdoutPath | Out-String).Trim() } else { "" }
    StandardError = if (Test-Path $stderrPath) { (Get-Content $stderrPath | Out-String).Trim() } else { "" }
} | ConvertTo-Json -Depth 3
