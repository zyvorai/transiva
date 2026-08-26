#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

# HyperSDK Coverage Report Generator
# Generate detailed test coverage reports and identify areas needing improvement

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
COVERAGE_FILE="${COVERAGE_FILE:-coverage.out}"
HTML_FILE="${HTML_FILE:-coverage.html}"
THRESHOLD="${COVERAGE_THRESHOLD:-70}"
PACKAGE="${PACKAGE:-./...}"

# Print usage
usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Generate detailed test coverage reports for HyperSDK.

OPTIONS:
    -h, --help              Show this help message
    -p, --package PKG       Generate coverage for specific package (default: ./...)
    -t, --threshold PCT     Coverage threshold percentage (default: 70)
    -o, --output FILE       Output coverage file (default: coverage.out)
    --html FILE             HTML output file (default: coverage.html)
    --open                  Open HTML report in browser
    --api                   Generate report for API package only
    --json                  Output coverage in JSON format
    --summary               Show only summary (no detailed breakdown)

EXAMPLES:
    $0                          # Generate full coverage report
    $0 --api --open             # Generate API coverage and open in browser
    $0 --threshold 80           # Set 80% coverage threshold
    $0 --package ./daemon/api   # Coverage for specific package

ENVIRONMENT VARIABLES:
    COVERAGE_FILE        Coverage output file (default: coverage.out)
    COVERAGE_THRESHOLD   Minimum coverage percentage (default: 70)
EOF
}

# Parse arguments
OPEN_HTML=false
OUTPUT_JSON=false
SUMMARY_ONLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -p|--package)
            PACKAGE="$2"
            shift 2
            ;;
        -t|--threshold)
            THRESHOLD="$2"
            shift 2
            ;;
        -o|--output)
            COVERAGE_FILE="$2"
            shift 2
            ;;
        --html)
            HTML_FILE="$2"
            shift 2
            ;;
        --open)
            OPEN_HTML=true
            shift
            ;;
        --api)
            PACKAGE="./daemon/api"
            shift
            ;;
        --json)
            OUTPUT_JSON=true
            shift
            ;;
        --summary)
            SUMMARY_ONLY=true
            shift
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
    esac
done

# Print header
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}HyperSDK Coverage Report${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Package:   ${CYAN}${PACKAGE}${NC}"
echo -e "Threshold: ${CYAN}${THRESHOLD}%${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Generate coverage
echo -e "${YELLOW}➜ Generating coverage data...${NC}"
if go test -coverprofile=${COVERAGE_FILE} ${PACKAGE}; then
    echo -e "${GREEN}✅ Coverage data generated${NC}"
else
    echo -e "${RED}❌ Failed to generate coverage${NC}"
    exit 1
fi

# Get total coverage
TOTAL_COVERAGE=$(go tool cover -func=${COVERAGE_FILE} | grep total | awk '{print $3}' | sed 's/%//')

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Coverage Summary${NC}"
echo -e "${BLUE}========================================${NC}"

# Print total coverage with color based on threshold
if (( $(echo "$TOTAL_COVERAGE >= $THRESHOLD" | bc -l) )); then
    echo -e "Total Coverage: ${GREEN}${TOTAL_COVERAGE}%${NC} ✅"
    PASSED=true
else
    echo -e "Total Coverage: ${RED}${TOTAL_COVERAGE}%${NC} ❌"
    echo -e "Below threshold of ${YELLOW}${THRESHOLD}%${NC}"
    PASSED=false
fi

if [[ "$SUMMARY_ONLY" == "false" ]]; then
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Coverage by Package${NC}"
    echo -e "${BLUE}========================================${NC}"

    # Get coverage by package
    go tool cover -func=${COVERAGE_FILE} | grep -v "total:" | awk '{
        package = $1
        sub(/\/[^/]+$/, "", package)
        coverage[package] += $3
        count[package]++
    }
    END {
        for (pkg in coverage) {
            avg = coverage[pkg] / count[pkg]
            printf "%-50s %6.1f%%\n", pkg, avg
        }
    }' | sort -t':' -k2 -rn | while read line; do
        COV=$(echo "$line" | awk '{print $NF}' | sed 's/%//')
        PKG=$(echo "$line" | awk '{$NF=""; print $0}')

        if (( $(echo "$COV >= 80" | bc -l) )); then
            echo -e "${GREEN}${PKG} ${COV}%${NC}"
        elif (( $(echo "$COV >= 60" | bc -l) )); then
            echo -e "${YELLOW}${PKG} ${COV}%${NC}"
        else
            echo -e "${RED}${PKG} ${COV}%${NC}"
        fi
    done

    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Functions with Low Coverage (<${THRESHOLD}%)${NC}"
    echo -e "${BLUE}========================================${NC}"

    # Find functions with low coverage
    LOW_COVERAGE=$(go tool cover -func=${COVERAGE_FILE} | grep -v "total:" | awk -v threshold="$THRESHOLD" '$3 < threshold {print $0}')

    if [[ -z "$LOW_COVERAGE" ]]; then
        echo -e "${GREEN}No functions below ${THRESHOLD}% coverage!${NC}"
    else
        echo "$LOW_COVERAGE" | head -20 | while read line; do
            COV=$(echo "$line" | awk '{print $3}' | sed 's/%//')
            FUNC=$(echo "$line" | awk '{$NF=""; print $0}')

            if (( $(echo "$COV == 0" | bc -l) )); then
                echo -e "${RED}${FUNC} ${COV}%${NC}"
            else
                echo -e "${YELLOW}${FUNC} ${COV}%${NC}"
            fi
        done

        # Count total low coverage functions
        TOTAL_LOW=$(echo "$LOW_COVERAGE" | wc -l)
        if [[ $TOTAL_LOW -gt 20 ]]; then
            echo ""
            echo -e "${YELLOW}... and $((TOTAL_LOW - 20)) more functions${NC}"
        fi
    fi

    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Top 10 Well-Covered Functions${NC}"
    echo -e "${BLUE}========================================${NC}"

    go tool cover -func=${COVERAGE_FILE} | grep -v "total:" | sort -t':' -k3 -rn | head -10 | while read line; do
        COV=$(echo "$line" | awk '{print $3}' | sed 's/%//')
        FUNC=$(echo "$line" | awk '{$NF=""; print $0}')
        echo -e "${GREEN}${FUNC} ${COV}%${NC}"
    done
fi

# Generate HTML report
echo ""
echo -e "${YELLOW}➜ Generating HTML report...${NC}"
if go tool cover -html=${COVERAGE_FILE} -o ${HTML_FILE}; then
    echo -e "${GREEN}✅ HTML report generated: ${HTML_FILE}${NC}"

    # Open in browser if requested
    if [[ "$OPEN_HTML" == "true" ]]; then
        if command -v xdg-open &> /dev/null; then
            xdg-open ${HTML_FILE}
        elif command -v open &> /dev/null; then
            open ${HTML_FILE}
        else
            echo -e "${YELLOW}⚠️  Please open ${HTML_FILE} in your browser${NC}"
        fi
    fi
else
    echo -e "${RED}❌ Failed to generate HTML report${NC}"
fi

# JSON output if requested
if [[ "$OUTPUT_JSON" == "true" ]]; then
    echo ""
    echo -e "${YELLOW}➜ Generating JSON output...${NC}"

    cat > coverage.json <<EOF
{
  "package": "${PACKAGE}",
  "total_coverage": ${TOTAL_COVERAGE},
  "threshold": ${THRESHOLD},
  "passed": $([ "$PASSED" = true ] && echo "true" || echo "false"),
  "coverage_file": "${COVERAGE_FILE}",
  "html_file": "${HTML_FILE}",
  "generated_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

    echo -e "${GREEN}✅ JSON output saved to coverage.json${NC}"
fi

# Print footer
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Report Complete${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Files generated
echo -e "Files generated:"
echo -e "  • ${CYAN}${COVERAGE_FILE}${NC} - Coverage data"
echo -e "  • ${CYAN}${HTML_FILE}${NC} - HTML report"
if [[ "$OUTPUT_JSON" == "true" ]]; then
    echo -e "  • ${CYAN}coverage.json${NC} - JSON summary"
fi

echo ""

# Exit with appropriate code
if [[ "$PASSED" == "true" ]]; then
    echo -e "${GREEN}✅ Coverage threshold met!${NC}"
    exit 0
else
    echo -e "${RED}❌ Coverage below threshold${NC}"
    echo -e "Current: ${TOTAL_COVERAGE}%, Required: ${THRESHOLD}%"
    exit 1
fi
