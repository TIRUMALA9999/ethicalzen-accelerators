#!/bin/bash
# ============================================================================
# EthicalZen — Live Metrics Dashboard
# Polls gateway + sidecar endpoints and displays a live-updating terminal view.
#
# Usage:
#   ./metrics-dashboard.sh                  # Refresh every 2s
#   ./metrics-dashboard.sh --interval 5     # Refresh every 5s
#   ./metrics-dashboard.sh --once           # Print once and exit
# ============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

HOST="${GATEWAY_HOST:-localhost}"
GW_PORT="${GATEWAY_PORT:-8080}"
SG_PORT="${SG_PORT:-3001}"
METRICS_PORT="${METRICS_PORT:-9090}"
REPORT_DIR="${SCRIPT_DIR}/reports"
INTERVAL=2
ONCE=false

for arg in "$@"; do
  case "$arg" in
    --interval=*) INTERVAL="${arg#*=}" ;;
    --once)       ONCE=true ;;
  esac
done

# ── Colors ─────────────────────────────────────────────────────
G='\033[0;32m'   # green
R='\033[0;31m'   # red
Y='\033[1;33m'   # yellow
C='\033[0;36m'   # cyan
B='\033[1m'      # bold
D='\033[2m'      # dim
N='\033[0m'      # reset

# ── Box-drawing helpers ────────────────────────────────────────
WIDTH=66
hline()  { printf "║  "; printf '─%.0s' $(seq 1 $((WIDTH-4))); printf "  ║\n"; }
blank()  { printf "║  %-$((WIDTH-4))s  ║\n" ""; }
row()    { printf "║  %-$((WIDTH-4))s  ║\n" "$1"; }
rowc()   { printf "║  "; printf "$1"; printf "%*s" $((WIDTH - 4 - ${2:-0})) ""; printf "  ║\n"; }
top()    { printf "╔"; printf '═%.0s' $(seq 1 $WIDTH); printf "╗\n"; }
mid()    { printf "╠"; printf '═%.0s' $(seq 1 $WIDTH); printf "╣\n"; }
bot()    { printf "╚"; printf '═%.0s' $(seq 1 $WIDTH); printf "╝\n"; }

# ── Data fetch ─────────────────────────────────────────────────
fetch_data() {
  GW_HEALTH=$(curl -sf "http://${HOST}:${GW_PORT}/health" 2>/dev/null || echo '{}')
  SG_HEALTH=$(curl -sf "http://${HOST}:${SG_PORT}/health" 2>/dev/null || echo '{}')
  PROM=$(curl -sf "http://${HOST}:${METRICS_PORT}/metrics" 2>/dev/null || echo '')

  # Gateway health fields
  GW_STATUS=$(echo "$GW_HEALTH" | jq -r '.status // "offline"' 2>/dev/null)
  GW_VERSION=$(echo "$GW_HEALTH" | jq -r '.version // "?"' 2>/dev/null)
  GW_CIRCUIT=$(echo "$GW_HEALTH" | jq -r '.circuit_state // "?"' 2>/dev/null)
  GW_CFAILS=$(echo "$GW_HEALTH" | jq -r '.circuit_stats.consecutive_failures // 0' 2>/dev/null)
  GW_CACHED=$(echo "$GW_HEALTH" | jq -r '.guardrails_cached // 0' 2>/dev/null)
  GW_SIDECAR=$(echo "$GW_HEALTH" | jq -r '.sidecar_status // "?"' 2>/dev/null)

  # Sidecar health fields
  SG_STATUS=$(echo "$SG_HEALTH" | jq -r '.status // "offline"' 2>/dev/null)
  SG_READY=$(echo "$SG_HEALTH" | jq -r '.ready // false' 2>/dev/null)
  SG_MODEL=$(echo "$SG_HEALTH" | jq -r '.modelLoaded // false' 2>/dev/null)
  SG_CACHED=$(echo "$SG_HEALTH" | jq -r '.guardrailsCached // 0' 2>/dev/null)

  # Prometheus metrics
  GOROUTINES=$(echo "$PROM" | grep '^go_goroutines ' | awk '{print $2}')
  MEM_BYTES=$(echo "$PROM" | grep '^go_memstats_alloc_bytes ' | awk '{print $2}')
  GC_CYCLES=$(echo "$PROM" | grep '^go_gc_duration_seconds_count ' | awk '{print $2}')
  PROC_MEM=$(echo "$PROM" | grep '^process_resident_memory_bytes ' | awk '{print $2}')
  MEM_MB=$(python3 -c "print(f'{${MEM_BYTES:-0}/1048576:.1f}')" 2>/dev/null || echo "?")
  RSS_MB=$(python3 -c "print(f'{${PROC_MEM:-0}/1048576:.1f}')" 2>/dev/null || echo "?")

  # Guardrail type breakdown from sidecar
  GR_LIST=$(curl -sf "http://${HOST}:${SG_PORT}/guardrails" 2>/dev/null || echo '{"guardrails":[]}')
  GR_REGEX=$(echo "$GR_LIST" | jq '[(.guardrails // .)[] | select(.type=="regex")] | length' 2>/dev/null || echo 0)
  GR_SMART=$(echo "$GR_LIST" | jq '[(.guardrails // .)[] | select(.type=="smart_guardrail")] | length' 2>/dev/null || echo 0)
  GR_HYBRID=$(echo "$GR_LIST" | jq '[(.guardrails // .)[] | select(.type=="hybrid")] | length' 2>/dev/null || echo 0)
  GR_KEYWORD=$(echo "$GR_LIST" | jq '[(.guardrails // .)[] | select(.type=="keyword")] | length' 2>/dev/null || echo 0)
  GR_DLM=$(echo "$GR_LIST" | jq '[(.guardrails // .)[] | select(.type=="dlm_kernel")] | length' 2>/dev/null || echo 0)

  NOW=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
}

# ── Status colorizer ───────────────────────────────────────────
status_color() {
  case "$1" in
    healthy|closed|true|ready) echo -e "${G}${B}$1${N}" ;;
    offline|open|false)        echo -e "${R}${B}$1${N}" ;;
    *)                         echo -e "${Y}$1${N}" ;;
  esac
}

# Visible length of a string (strip ANSI codes)
vlen() { echo -ne "$1" | sed 's/\x1B\[[0-9;]*m//g' | wc -c | tr -d ' '; }

# ── Render dashboard ───────────────────────────────────────────
render() {
  # Build colored status strings
  local gw_s=$(status_color "$GW_STATUS")
  local sg_s=$(status_color "$SG_STATUS")
  local cb_s=$(status_color "$GW_CIRCUIT")
  local rdy_s=$(status_color "$SG_READY")
  local mdl_s=$(status_color "$SG_MODEL")

  clear
  top
  printf "║  ${B}${C}%-$((WIDTH-4))s${N}  ║\n" "EthicalZen — Live Metrics Dashboard"
  mid

  # ── System Health ──
  row ""
  rowc "${B}SYSTEM HEALTH${N}" 13
  row ""
  row "  Component       Status          Detail"
  row "  ───────────────────────────────────────────────────────"
  printf "║  %-18s" "  Gateway"
  printf "%-22b" "$gw_s"
  printf "v%-10s" "$GW_VERSION"
  printf "%*s║\n" $((WIDTH - 52)) ""

  printf "║  %-18s" "  Sidecar"
  printf "%-22b" "$sg_s"
  printf "Ready: %-5b" "$rdy_s"
  printf "%*s║\n" $((WIDTH - 49)) ""

  printf "║  %-18s" "  Circuit Breaker"
  printf "%-22b" "$cb_s"
  printf "Failures: %-3s" "$GW_CFAILS"
  printf "%*s║\n" $((WIDTH - 55)) ""

  printf "║  %-18s" "  Embedding Model"
  printf "%-22b" "$mdl_s"
  printf "%-10s" ""
  printf "%*s║\n" $((WIDTH - 52)) ""

  row ""

  # ── Guardrails + Runtime side by side ──
  row "  GUARDRAILS                      RUNTIME"
  row "  ───────────────────────────     ─────────────────────────"
  printf "║    Loaded: ${B}%-3s${N} guardrails           Goroutines:  %-16s  ║\n" "$SG_CACHED" "${GOROUTINES:-?}"
  printf "║  %-32s  %-28s  ║\n" "    regex:         ${GR_REGEX}" "Heap Memory:  ${MEM_MB} MB"
  printf "║  %-32s  %-28s  ║\n" "    smart:         ${GR_SMART}" "RSS Memory:   ${RSS_MB} MB"
  printf "║  %-32s  %-28s  ║\n" "    hybrid:        ${GR_HYBRID}" "GC Cycles:    ${GC_CYCLES:-?}"
  printf "║  %-32s  %-28s  ║\n" "    keyword:       ${GR_KEYWORD}" ""
  printf "║  %-32s  %-28s  ║\n" "    dlm_kernel:    ${GR_DLM}" ""
  row ""

  # ── Test Results ──
  row "  LATEST TEST RESULTS"
  row "  ───────────────────────────────────────────────────────"
  printf "║  %-26s %6s %6s %6s %7s  ║\n" "  Suite" "Pass" "Fail" "Skip" "Rate"

  local found=0
  for report in "${REPORT_DIR}"/*.json; do
    [ -f "$report" ] || continue
    found=1
    local rname=$(jq -r '.report // "?"' "$report" 2>/dev/null)
    local rpassed=$(jq -r '.passed // 0' "$report" 2>/dev/null)
    local rfailed=$(jq -r '.failed // 0' "$report" 2>/dev/null)
    local rskipped=$(jq -r '.skipped // 0' "$report" 2>/dev/null)
    local rtotal=$(jq -r '.total_tests // 0' "$report" 2>/dev/null)
    local rrate=$(( rtotal > 0 ? (rpassed * 100 / rtotal) : 0 ))

    local fcolor="${G}"
    [ "$rfailed" -gt 0 ] && fcolor="${R}"

    printf "║  %-26s " "  ${rname}"
    printf "${G}%5s${N} " "$rpassed"
    printf "${fcolor}%5s${N} " "$rfailed"
    printf "${Y}%5s${N} " "$rskipped"
    if [ "$rrate" -eq 100 ]; then
      printf "${G}%5s%%${N}" "$rrate"
    elif [ "$rrate" -ge 90 ]; then
      printf "${Y}%5s%%${N}" "$rrate"
    else
      printf "${R}%5s%%${N}" "$rrate"
    fi
    printf "  ║\n"
  done

  if [ "$found" -eq 0 ]; then
    row "    No test reports yet. Run: ./test-guardrails.sh"
  fi

  row ""

  # ── Footer ──
  mid
  printf "║  ${D}Updated: ${NOW}    Refresh: ${INTERVAL}s    [Ctrl+C to exit]${N}"
  printf "%*s║\n" $((WIDTH - 60)) ""
  bot
}

# ── Main loop ──────────────────────────────────────────────────
trap 'tput cnorm 2>/dev/null; echo ""; exit 0' INT TERM

tput civis 2>/dev/null  # hide cursor (graceful if no terminal)

if [ "$ONCE" = true ]; then
  fetch_data
  render
  tput cnorm 2>/dev/null
  exit 0
fi

while true; do
  fetch_data
  render
  sleep "$INTERVAL"
done
