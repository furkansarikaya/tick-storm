#!/bin/bash
# Security vulnerability scanning script for TickStorm

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Running TickStorm Security Vulnerability Scan...${NC}"
echo "=================================================="

# Check if required tools are installed
check_tool() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}❌ $1 is not installed${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ $1 is available${NC}"
    return 0
}

echo -e "${BLUE}Checking security tools...${NC}"
TOOLS_AVAILABLE=true

if ! check_tool "govulncheck"; then
    echo -e "${YELLOW}Installing govulncheck...${NC}"
    go install golang.org/x/vuln/cmd/govulncheck@latest || TOOLS_AVAILABLE=false
fi

if ! check_tool "docker"; then
    echo -e "${RED}Docker is required but not available${NC}"
    TOOLS_AVAILABLE=false
fi

if [ "$TOOLS_AVAILABLE" = false ]; then
    echo -e "${RED}❌ Required tools are missing. Please install them first.${NC}"
    exit 1
fi

echo ""

# Go vulnerability check
echo -e "${BLUE}🔍 Checking Go vulnerabilities...${NC}"
echo "----------------------------------"
if govulncheck ./...; then
    echo -e "${GREEN}✅ No Go vulnerabilities found${NC}"
else
    echo -e "${RED}❌ Go vulnerabilities detected${NC}"
fi

echo ""

# Check if Docker image exists
if docker image inspect tickstorm:latest &> /dev/null; then
    echo -e "${BLUE}🐳 Scanning Docker image for vulnerabilities...${NC}"
    echo "------------------------------------------------"
    
    # Use Trivy if available, otherwise skip Docker scan
    if command -v trivy &> /dev/null; then
        trivy image tickstorm:latest
    else
        echo -e "${YELLOW}⚠️  Trivy not available, skipping Docker image scan${NC}"
        echo -e "${YELLOW}   Install with: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  Docker image 'tickstorm:latest' not found, skipping Docker scan${NC}"
    echo -e "${YELLOW}   Build image first with: docker build -t tickstorm:latest .${NC}"
fi

echo ""

# Dependency audit
echo -e "${BLUE}📦 Auditing Go module dependencies...${NC}"
echo "--------------------------------------"
if go list -json -m all > /tmp/go-deps.json; then
    echo -e "${GREEN}✅ Go module dependencies exported${NC}"
    
    # Check for known vulnerable packages (basic check)
    if grep -q "github.com/gin-gonic/gin" /tmp/go-deps.json; then
        echo -e "${YELLOW}⚠️  Found Gin framework - ensure latest version${NC}"
    fi
    
    # Check Go version
    GO_VERSION=$(go version | cut -d' ' -f3 | sed 's/go//')
    echo -e "${BLUE}Go version: ${GO_VERSION}${NC}"
    
    # Basic version check (Go 1.21+ recommended for security)
    if [[ "$GO_VERSION" < "1.21" ]]; then
        echo -e "${YELLOW}⚠️  Consider upgrading to Go 1.21+ for latest security fixes${NC}"
    else
        echo -e "${GREEN}✅ Go version is recent${NC}"
    fi
else
    echo -e "${RED}❌ Failed to export Go dependencies${NC}"
fi

echo ""

# Check for common security issues in code
echo -e "${BLUE}🔒 Checking for common security patterns...${NC}"
echo "-------------------------------------------"

# Check for hardcoded credentials (basic patterns)
if grep -r -n "password.*=" --include="*.go" . | grep -v "_test.go" | grep -v "example" | head -5; then
    echo -e "${YELLOW}⚠️  Potential hardcoded credentials found (review above)${NC}"
else
    echo -e "${GREEN}✅ No obvious hardcoded credentials found${NC}"
fi

# Check for SQL injection patterns
if grep -r -n "fmt.Sprintf.*SELECT\|fmt.Sprintf.*INSERT\|fmt.Sprintf.*UPDATE" --include="*.go" . | head -5; then
    echo -e "${YELLOW}⚠️  Potential SQL injection patterns found (review above)${NC}"
else
    echo -e "${GREEN}✅ No obvious SQL injection patterns found${NC}"
fi

# Check for insecure TLS configurations
if grep -r -n "InsecureSkipVerify.*true" --include="*.go" . | grep -v "_test.go" | head -5; then
    echo -e "${YELLOW}⚠️  Insecure TLS configurations found (review above)${NC}"
else
    echo -e "${GREEN}✅ No insecure TLS configurations found${NC}"
fi

echo ""

# Security configuration check
echo -e "${BLUE}⚙️  Checking security configurations...${NC}"
echo "---------------------------------------"

# Check if TLS is enabled by default
if grep -q "TLS_ENABLED.*true" README.md docker-compose.yml 2>/dev/null; then
    echo -e "${GREEN}✅ TLS appears to be enabled by default${NC}"
else
    echo -e "${YELLOW}⚠️  Verify TLS is enabled in production${NC}"
fi

# Check for authentication requirements
if grep -q "AUTH_USERNAME\|AUTH_PASSWORD" README.md docker-compose.yml 2>/dev/null; then
    echo -e "${GREEN}✅ Authentication configuration found${NC}"
else
    echo -e "${YELLOW}⚠️  Verify authentication is properly configured${NC}"
fi

echo ""

# Generate security report summary
echo -e "${BLUE}📊 Security Scan Summary${NC}"
echo "========================"
echo "Scan completed at: $(date)"
echo "Go version: $GO_VERSION"
echo "Project: TickStorm TCP Stream Server"

# Cleanup
rm -f /tmp/go-deps.json

echo ""
echo -e "${GREEN}✅ Security vulnerability scan completed${NC}"
echo -e "${BLUE}💡 Recommendations:${NC}"
echo "   1. Run this scan regularly (daily in CI/CD)"
echo "   2. Keep Go and dependencies updated"
echo "   3. Review any warnings above"
echo "   4. Consider adding Trivy for Docker image scanning"
echo "   5. Implement security testing in CI pipeline"
