#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test JWT token
JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwicm9sZXMiOlsiYWRtaW4iLCJ1c2VyIl0sImN1c3RvbSI6eyJ0ZW5hbnRfaWQiOiJ0ZW5hbnQtMTIzIn0sImlhdCI6MTUxNjIzOTAyMn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

# Base URL
BASE_URL="http://whoami.localhost"

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Traefik JWT Decoder Plugin Test Suite${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Function to print test header
print_test_header() {
    echo -e "${YELLOW}Test $1: $2${NC}"
    echo "--------------------------------------"
}

# Function to check if header exists in response
check_header() {
    local response="$1"
    local header_name="$2"
    local expected_value="$3"
    
    if echo "$response" | grep -qi "^${header_name}: ${expected_value}"; then
        echo -e "${GREEN}✓ ${header_name}: ${expected_value}${NC}"
        return 0
    else
        echo -e "${RED}✗ ${header_name}: ${expected_value} (NOT FOUND)${NC}"
        return 1
    fi
}

# Function to check if header does NOT exist in response
check_header_absent() {
    local response="$1"
    local header_name="$2"
    
    if ! echo "$response" | grep -qi "^${header_name}:"; then
        echo -e "${GREEN}✓ ${header_name} correctly absent${NC}"
        return 0
    else
        echo -e "${RED}✗ ${header_name} should not be present${NC}"
        return 1
    fi
}

# Test 1: Valid JWT with Bearer prefix
print_test_header "1" "Valid JWT with Bearer prefix"
RESPONSE=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" "$BASE_URL" 2>/dev/null)

if [ -z "$RESPONSE" ]; then
    echo -e "${RED}✗ No response received. Is the service running?${NC}"
    echo -e "${YELLOW}Run: docker-compose up -d && sleep 5${NC}"
    exit 1
fi

TEST_PASSED=true
check_header "$RESPONSE" "X-User-Id" "1234567890" || TEST_PASSED=false
check_header "$RESPONSE" "X-User-Email" "test@example.com" || TEST_PASSED=false
check_header "$RESPONSE" "X-User-Roles" "admin, user" || TEST_PASSED=false
check_header "$RESPONSE" "X-Tenant-Id" "tenant-123" || TEST_PASSED=false

if [ "$TEST_PASSED" = true ]; then
    echo -e "${GREEN}✓ Test 1 PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "${RED}✗ Test 1 FAILED${NC}"
    ((TESTS_FAILED++))
fi
echo ""

# Test 2: Request without JWT (continueOnError test)
print_test_header "2" "Request without JWT (continueOnError=true)"
RESPONSE=$(curl -s "$BASE_URL" 2>/dev/null)

TEST_PASSED=true
# Check HTTP status is 200 (not 401)
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL")
if [ "$HTTP_STATUS" = "200" ]; then
    echo -e "${GREEN}✓ HTTP Status: 200 (request passed through)${NC}"
else
    echo -e "${RED}✗ HTTP Status: $HTTP_STATUS (expected 200)${NC}"
    TEST_PASSED=false
fi

# Verify no JWT headers were injected
check_header_absent "$RESPONSE" "X-User-Id" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-User-Email" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-User-Roles" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-Tenant-Id" || TEST_PASSED=false

if [ "$TEST_PASSED" = true ]; then
    echo -e "${GREEN}✓ Test 2 PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "${RED}✗ Test 2 FAILED${NC}"
    ((TESTS_FAILED++))
fi
echo ""

# Test 3: Invalid JWT format
print_test_header "3" "Invalid JWT format (continueOnError=true)"
RESPONSE=$(curl -s -H "Authorization: Bearer invalid.token.here" "$BASE_URL" 2>/dev/null)

TEST_PASSED=true
# Check HTTP status is 200 (not 401)
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer invalid.token.here" "$BASE_URL")
if [ "$HTTP_STATUS" = "200" ]; then
    echo -e "${GREEN}✓ HTTP Status: 200 (request passed through)${NC}"
else
    echo -e "${RED}✗ HTTP Status: $HTTP_STATUS (expected 200)${NC}"
    TEST_PASSED=false
fi

# Verify no JWT headers were injected
check_header_absent "$RESPONSE" "X-User-Id" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-User-Email" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-User-Roles" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-Tenant-Id" || TEST_PASSED=false

if [ "$TEST_PASSED" = true ]; then
    echo -e "${GREEN}✓ Test 3 PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "${RED}✗ Test 3 FAILED${NC}"
    ((TESTS_FAILED++))
fi
echo ""

# Test 4: JWT without Bearer prefix (flexible prefix handling)
print_test_header "4" "JWT without 'Bearer ' prefix (flexible mode)"
RESPONSE=$(curl -s -H "Authorization: $JWT_TOKEN" "$BASE_URL" 2>/dev/null)

TEST_PASSED=true
# Check HTTP status is 200
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: $JWT_TOKEN" "$BASE_URL")
if [ "$HTTP_STATUS" = "200" ]; then
    echo -e "${GREEN}✓ HTTP Status: 200${NC}"
else
    echo -e "${RED}✗ HTTP Status: $HTTP_STATUS (expected 200)${NC}"
    TEST_PASSED=false
fi

# Plugin supports flexible prefix - headers should be injected even without Bearer prefix
echo -e "${BLUE}Note: Plugin accepts JWT with or without 'Bearer ' prefix${NC}"
check_header "$RESPONSE" "X-User-Id" "1234567890" || TEST_PASSED=false
check_header "$RESPONSE" "X-User-Email" "test@example.com" || TEST_PASSED=false
check_header "$RESPONSE" "X-User-Roles" "admin, user" || TEST_PASSED=false
check_header "$RESPONSE" "X-Tenant-Id" "tenant-123" || TEST_PASSED=false

if [ "$TEST_PASSED" = true ]; then
    echo -e "${GREEN}✓ Test 4 PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "${RED}✗ Test 4 FAILED${NC}"
    ((TESTS_FAILED++))
fi
echo ""

# Test 5: Malformed base64 JWT
print_test_header "5" "Malformed base64 JWT"
MALFORMED_JWT="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.INVALID_BASE64@@@.signature"
RESPONSE=$(curl -s -H "Authorization: Bearer $MALFORMED_JWT" "$BASE_URL" 2>/dev/null)

TEST_PASSED=true
# Check HTTP status is 200 (not 401)
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $MALFORMED_JWT" "$BASE_URL")
if [ "$HTTP_STATUS" = "200" ]; then
    echo -e "${GREEN}✓ HTTP Status: 200 (request passed through)${NC}"
else
    echo -e "${RED}✗ HTTP Status: $HTTP_STATUS (expected 200)${NC}"
    TEST_PASSED=false
fi

# Verify no JWT headers were injected
check_header_absent "$RESPONSE" "X-User-Id" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-User-Email" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-User-Roles" || TEST_PASSED=false
check_header_absent "$RESPONSE" "X-Tenant-Id" || TEST_PASSED=false

if [ "$TEST_PASSED" = true ]; then
    echo -e "${GREEN}✓ Test 5 PASSED${NC}"
    ((TESTS_PASSED++))
else
    echo -e "${RED}✗ Test 5 FAILED${NC}"
    ((TESTS_FAILED++))
fi
echo ""

# Summary
echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}======================================${NC}"
echo -e "Total Tests: $((TESTS_PASSED + TESTS_FAILED))"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo -e "${YELLOW}Plugin is working correctly.${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed.${NC}"
    echo ""
    echo -e "${YELLOW}Troubleshooting steps:${NC}"
    echo "1. Check Traefik logs: docker-compose logs traefik"
    echo "2. Verify plugin loaded: docker-compose logs traefik | grep -i plugin"
    echo "3. Check dynamic config: cat dynamic-config.yml"
    echo "4. Restart Traefik: docker-compose restart traefik"
    exit 1
fi
