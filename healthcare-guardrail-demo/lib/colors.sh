#!/bin/bash
# ============================================================================
# EthicalZen Demo — Shared Color & Formatting Library
# Source this file: source "$(dirname "$0")/lib/colors.sh"
# ============================================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# Counters (use :- to preserve across re-sources when run via master runner)
: ${TOTAL_TESTS:=0}
: ${PASSED_TESTS:=0}
: ${FAILED_TESTS:=0}
: ${SKIPPED_TESTS:=0}

# Narrate mode (set via --narrate flag)
NARRATE=${NARRATE:-false}

print_pass()  { echo -e "  ${GREEN}[PASS]${NC} $1"; TOTAL_TESTS=$((TOTAL_TESTS+1)); PASSED_TESTS=$((PASSED_TESTS+1)); }
print_fail()  { echo -e "  ${RED}[FAIL]${NC} $1"; TOTAL_TESTS=$((TOTAL_TESTS+1)); FAILED_TESTS=$((FAILED_TESTS+1)); }
print_skip()  { echo -e "  ${YELLOW}[SKIP]${NC} $1"; TOTAL_TESTS=$((TOTAL_TESTS+1)); SKIPPED_TESTS=$((SKIPPED_TESTS+1)); }
print_info()  { echo -e "  ${BLUE}[INFO]${NC} $1"; }
print_warn()  { echo -e "  ${YELLOW}[WARN]${NC} $1"; }
print_error() { echo -e "  ${RED}[ERROR]${NC} $1"; }

print_header() {
  echo ""
  echo -e "${BOLD}${CYAN}===============================================================================${NC}"
  echo -e "${BOLD}${CYAN}  $1${NC}"
  echo -e "${BOLD}${CYAN}===============================================================================${NC}"
  echo ""
}

print_subheader() {
  echo ""
  echo -e "${BOLD}  --- $1 ---${NC}"
  echo ""
}

print_step() {
  local step_num=$1
  shift
  echo ""
  echo -e "  ${BOLD}${BLUE}STEP ${step_num}:${NC} ${BOLD}$*${NC}"
  echo -e "  ${DIM}$(printf '%.0s-' {1..70})${NC}"
}

print_separator() {
  echo -e "  ${DIM}$(printf '%.0s-' {1..70})${NC}"
}

print_summary() {
  echo ""
  echo -e "${BOLD}${CYAN}===============================================================================${NC}"
  echo -e "${BOLD}${CYAN}  TEST SUMMARY${NC}"
  echo -e "${BOLD}${CYAN}===============================================================================${NC}"
  echo ""
  echo -e "  Total:   ${BOLD}${TOTAL_TESTS}${NC}"
  echo -e "  Passed:  ${GREEN}${PASSED_TESTS}${NC}"
  echo -e "  Failed:  ${RED}${FAILED_TESTS}${NC}"
  echo -e "  Skipped: ${YELLOW}${SKIPPED_TESTS}${NC}"
  echo ""
  if [ "$FAILED_TESTS" -eq 0 ]; then
    echo -e "  ${GREEN}${BOLD}ALL TESTS PASSED${NC}"
  else
    echo -e "  ${RED}${BOLD}${FAILED_TESTS} TESTS FAILED${NC}"
  fi
  echo ""
  echo -e "${BOLD}${CYAN}===============================================================================${NC}"
}

narrate() {
  if [ "$NARRATE" = "true" ]; then
    echo ""
    echo -e "  ${DIM}$1${NC}"
    echo ""
    echo -e "  ${YELLOW}Press Enter to continue...${NC}"
    read -r
  fi
}

# Cross-platform millisecond timestamp (macOS date doesn't support %3N)
millis() {
  python3 -c 'import time; print(int(time.time()*1000))'
}

# Parse --narrate flag from any script's arguments
parse_common_args() {
  for arg in "$@"; do
    case "$arg" in
      --narrate) NARRATE=true ;;
    esac
  done
}
