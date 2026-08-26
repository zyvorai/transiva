#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

# HyperSDK Test Runner Script
# Runs tests intelligently with proper filtering and reporting

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TIMEOUT="${TEST_TIMEOUT:-60s}"
COVERAGE_FILE="${COVERAGE_FILE:-coverage.out}"
VERBOSE="${VERBOSE:-false}"

# Print usage
usage() {
    cat <<EOF
Usage: $0 [OPTIONS] [PACKAGE]

Run Go tests for HyperSDK with intelligent filtering and reporting.

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Run tests with verbose output
    -c, --coverage      Generate coverage report
    -s, --short         Run only short tests (fast)
    -r, --race          Run with race detector
    -t, --timeout SEC   Set test timeout (default: 60s)
    -p, --package PKG   Run tests for specific package
    -f, --fast          Run fast tests (no race detector, short timeout)
    --api               Run only API tests
    --unit              Run only unit tests (no integration)
    --html              Open coverage report in browser

EXAMPLES:
    $0                              # Run all tests
    $0 --coverage                   # Run all tests with coverage
    $0 --api                        # Run only API handler tests
    $0 --package ./daemon/api       # Run tests for specific package
    $0 --fast --api                 # Quick API test run
    $0 --coverage --html            # Generate and open coverage report

ENVIRONMENT VARIABLES:
    TEST_TIMEOUT     Test timeout (default: 60s)
    COVERAGE_FILE    Coverage output file (default: coverage.out)
    VERBOSE          Enable verbose output (true/false)
EOF
}

# Parse arguments
PACKAGE="./..."
RUN_COVERAGE=false
RUN_SHORT=false
RUN_RACE=false
OPEN_HTML=false
TEST_FILTER=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -c|--coverage)
            RUN_COVERAGE=true
            shift
            ;;
        -s|--short)
            RUN_SHORT=true
            TIMEOUT="30s"
            shift
            ;;
        -r|--race)
            RUN_RACE=true
            shift
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -p|--package)
            PACKAGE="$2"
            shift 2
            ;;
        -f|--fast)
            RUN_SHORT=true
            RUN_RACE=false
            TIMEOUT="30s"
            shift
            ;;
        --api)
            PACKAGE="./daemon/api"
            TEST_FILTER="-run TestHandle"
            shift
            ;;
        --unit)
            TEST_FILTER="-short"
            RUN_SHORT=true
            shift
            ;;
        --html)
            OPEN_HTML=true
            RUN_COVERAGE=true
            shift
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
    esac
done

# Build test command
TEST_CMD="go test"
TEST_ARGS="-timeout=${TIMEOUT}"

if [[ "$VERBOSE" == "true" ]]; then
    TEST_ARGS="$TEST_ARGS -v"
fi

if [[ "$RUN_COVERAGE" == "true" ]]; then
    TEST_ARGS="$TEST_ARGS -coverprofile=${COVERAGE_FILE}"
fi

if [[ "$RUN_SHORT" == "true" ]]; then
    TEST_ARGS="$TEST_ARGS -short"
fi

if [[ "$RUN_RACE" == "true" ]]; then
    TEST_ARGS="$TEST_ARGS -race"
fi

if [[ -n "$TEST_FILTER" ]]; then
    TEST_ARGS="$TEST_ARGS $TEST_FILTER"
fi

# Print configuration
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}HyperSDK Test Runner${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Package:  ${GREEN}${PACKAGE}${NC}"
echo -e "Timeout:  ${YELLOW}${TIMEOUT}${NC}"
echo -e "Coverage: ${YELLOW}${RUN_COVERAGE}${NC}"
echo -e "Race:     ${YELLOW}${RUN_RACE}${NC}"
echo -e "Short:    ${YELLOW}${RUN_SHORT}${NC}"
if [[ -n "$TEST_FILTER" ]]; then
    echo -e "Filter:   ${YELLOW}${TEST_FILTER}${NC}"
fi
echo -e "${BLUE}========================================${NC}"
echo ""

# Run tests
echo -e "${BLUE}Running tests...${NC}"
echo "Command: $TEST_CMD $TEST_ARGS $PACKAGE"
echo ""

if $TEST_CMD $TEST_ARGS $PACKAGE; then
    echo ""
    echo -e "${GREEN}✅ All tests passed!${NC}"

    # Show coverage if requested
    if [[ "$RUN_COVERAGE" == "true" ]]; then
        echo ""
        echo -e "${BLUE}========================================${NC}"
        echo -e "${BLUE}Coverage Summary${NC}"
        echo -e "${BLUE}========================================${NC}"
        go tool cover -func=${COVERAGE_FILE} | tail -1
        echo ""

        # Open HTML if requested
        if [[ "$OPEN_HTML" == "true" ]]; then
            echo -e "${BLUE}Generating HTML coverage report...${NC}"
            go tool cover -html=${COVERAGE_FILE} -o coverage.html
            echo -e "${GREEN}Coverage report saved to coverage.html${NC}"

            # Try to open in browser
            if command -v xdg-open &> /dev/null; then
                xdg-open coverage.html
            elif command -v open &> /dev/null; then
                open coverage.html
            else
                echo -e "${YELLOW}Please open coverage.html in your browser${NC}"
            fi
        fi
    fi

    exit 0
else
    echo ""
    echo -e "${RED}❌ Tests failed!${NC}"
    echo ""

    # If coverage was generated, show it anyway
    if [[ "$RUN_COVERAGE" == "true" ]] && [[ -f "$COVERAGE_FILE" ]]; then
        echo -e "${BLUE}Coverage Summary (from failed run):${NC}"
        go tool cover -func=${COVERAGE_FILE} | tail -1
        echo ""
    fi

    exit 1
fi
