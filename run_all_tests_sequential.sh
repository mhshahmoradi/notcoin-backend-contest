#!/bin/bash

# Sequential Test Runner Script
# This script runs all k6 tests sequentially with fresh database instances

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

wait_for_services() {
    print_status "Waiting for services to be ready..."
    
    print_status "Waiting for API service..."
    while ! curl -s http://localhost:8032/health >/dev/null 2>&1; do
        sleep 1
    done
    
    sleep 5
    print_success "All services are ready!"
}

run_test() {
    local test_name="$1"
    local test_file="$2"
    
    echo ""
    echo "========================================"
    print_status "Starting Test: $test_name"
    echo "========================================"
    
    print_status "Starting Docker Compose..."
    docker compose up -d
    
    wait_for_services
    
    print_status "Running k6 test: $test_file"
    if k6 run "$test_file"; then
        print_success "Test '$test_name' completed successfully!"
        return 0
    else
        print_error "Test '$test_name' failed!"
        return 1
    fi
}

cleanup_test() {
    print_status "Stopping Docker Compose and cleaning up data..."
    docker compose down -v --remove-orphans
    
    print_status "Removing any leftover volumes..."
    docker volume prune -f >/dev/null 2>&1 || true
    
    sleep 3
    
    print_success "Cleanup completed"
    echo ""
}

main() {
    print_status "Starting sequential k6 test execution..."
    print_status "Each test will run with a fresh database instance"
    echo ""
    
    if ! command -v k6 &> /dev/null; then
        print_error "k6 is not installed. Please install k6 first."
        print_status "Install k6 with: sudo apt-get install k6  # or brew install k6 on macOS"
        exit 1
    fi
    
    if [[ ! -f "docker-compose.yml" ]]; then
        print_error "docker-compose.yml not found in current directory"
        exit 1
    fi
    
    print_status "Initial cleanup..."
    docker compose down -v --remove-orphans >/dev/null 2>&1 || true
    docker volume prune -f >/dev/null 2>&1 || true
    
    declare -a tests=(
        "Race Condition Test|tests/k6/test1_race_condition.js"
        "User Limits Test|tests/k6/test2_user_limits.js"
        "Time To Buy All Item Test|tests/k6/test3_time_to_buy_all.js"
        "Sale Limit Test|tests/k6/test4_sale_limit.js"
    )
    
    declare -a results=()
    
    for test_config in "${tests[@]}"; do
        IFS='|' read -r test_name test_file <<< "$test_config"
        
        if [[ ! -f "$test_file" ]]; then
            print_warning "Test file '$test_file' not found, skipping..."
            results+=("$test_name: SKIPPED")
            continue
        fi
        
        if run_test "$test_name" "$test_file"; then
            results+=("$test_name: PASSED")
        else
            results+=("$test_name: FAILED")
        fi
        
        cleanup_test
    done
    
    print_status "Final cleanup..."
    docker compose down -v --remove-orphans >/dev/null 2>&1 || true
    docker volume prune -f >/dev/null 2>&1 || true
    
    echo ""
    echo "========================================"
    print_status "TEST EXECUTION SUMMARY"
    echo "========================================"
    
    for result in "${results[@]}"; do
        if [[ "$result" == *"PASSED"* ]]; then
            print_success "$result"
        elif [[ "$result" == *"FAILED"* ]]; then
            print_error "$result"
        else
            print_warning "$result"
        fi
    done
    
    echo ""
    print_success "All tests completed!"
}

trap 'print_warning "Script interrupted! Cleaning up..."; docker compose down -v --remove-orphans >/dev/null 2>&1 || true; exit 1' INT TERM

main "$@"