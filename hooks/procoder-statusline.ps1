# procoder — statusline badge (Windows).
$ErrorActionPreference = 'SilentlyContinue'

$configDir = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $HOME '.claude' }
$levelFile = Join-Path $configDir '.procoder-active'

if (-not (Test-Path $levelFile)) { exit 0 }

$level = (Get-Content $levelFile -Raw).Trim().ToLower()
if ($level -in @('pragmatic', 'strict', 'paranoid')) {
    Write-Output ("[PROCODER:{0}]" -f $level.ToUpper())
}
