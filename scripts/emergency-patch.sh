#!/bin/bash
# Emergency security patch deployment script for TickStorm

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE=${NAMESPACE:-default}
DEPLOYMENT_NAME=${DEPLOYMENT_NAME:-tickstorm}
TIMEOUT=${TIMEOUT:-600}

# Parse command line arguments
PATCH_VERSION=""
EMERGENCY_REASON=""
ENVIRONMENT=""

usage() {
    echo "Usage: $0 -v <patch_version> -r <reason> -e <environment>"
    echo ""
    echo "Options:"
    echo "  -v, --version     Patch version (e.g., v1.2.3-security-001)"
    echo "  -r, --reason      Emergency reason (e.g., 'CVE-2024-1234 RCE vulnerability')"
    echo "  -e, --environment Target environment (staging|production)"
    echo "  -h, --help        Show this help message"
    echo ""
    echo "Example:"
    echo "  $0 -v v1.2.3-security-001 -r 'CVE-2024-1234 critical RCE' -e staging"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            PATCH_VERSION="$2"
            shift 2
            ;;
        -r|--reason)
            EMERGENCY_REASON="$2"
            shift 2
            ;;
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate required parameters
if [[ -z "$PATCH_VERSION" || -z "$EMERGENCY_REASON" || -z "$ENVIRONMENT" ]]; then
    echo -e "${RED}❌ Missing required parameters${NC}"
    usage
fi

# Validate environment
if [[ "$ENVIRONMENT" != "staging" && "$ENVIRONMENT" != "production" ]]; then
    echo -e "${RED}❌ Environment must be 'staging' or 'production'${NC}"
    exit 1
fi

# Emergency deployment confirmation
echo -e "${RED}🚨 EMERGENCY SECURITY PATCH DEPLOYMENT 🚨${NC}"
echo "=================================================="
echo -e "${BLUE}Patch Version:${NC} $PATCH_VERSION"
echo -e "${BLUE}Environment:${NC} $ENVIRONMENT"
echo -e "${BLUE}Reason:${NC} $EMERGENCY_REASON"
echo -e "${BLUE}Timestamp:${NC} $(date)"
echo ""

if [[ "$ENVIRONMENT" == "production" ]]; then
    echo -e "${YELLOW}⚠️  WARNING: This will deploy to PRODUCTION environment!${NC}"
    echo -e "${YELLOW}⚠️  Ensure staging deployment was successful first.${NC}"
    echo ""
    read -p "Type 'EMERGENCY' to confirm production deployment: " confirmation
    if [[ "$confirmation" != "EMERGENCY" ]]; then
        echo -e "${RED}❌ Production deployment cancelled${NC}"
        exit 1
    fi
fi

# Log emergency deployment
log_emergency() {
    local status=$1
    local message=$2
    echo "$(date): [$status] Emergency patch $PATCH_VERSION - $message" >> emergency-deployments.log
}

# Pre-deployment checks
echo -e "${BLUE}🔍 Running pre-deployment checks...${NC}"
echo "------------------------------------"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}❌ kubectl is not available${NC}"
    exit 1
fi

# Check cluster connectivity
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}❌ Cannot connect to Kubernetes cluster${NC}"
    exit 1
fi

# Check if deployment exists
if ! kubectl get deployment $DEPLOYMENT_NAME -n $NAMESPACE &> /dev/null; then
    echo -e "${RED}❌ Deployment $DEPLOYMENT_NAME not found in namespace $NAMESPACE${NC}"
    exit 1
fi

# Get current image for rollback
CURRENT_IMAGE=$(kubectl get deployment $DEPLOYMENT_NAME -n $NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].image}')
echo -e "${BLUE}Current image:${NC} $CURRENT_IMAGE"

# Check if new image exists
NEW_IMAGE="tickstorm:$PATCH_VERSION"
if ! docker image inspect $NEW_IMAGE &> /dev/null; then
    echo -e "${YELLOW}⚠️  Image $NEW_IMAGE not found locally, assuming it's in registry${NC}"
fi

echo -e "${GREEN}✅ Pre-deployment checks passed${NC}"
echo ""

# Build emergency patch image (if Dockerfile exists)
if [[ -f "Dockerfile" ]]; then
    echo -e "${BLUE}🔨 Building emergency patch image...${NC}"
    echo "-------------------------------------"
    
    # Tag with emergency suffix
    EMERGENCY_TAG="$PATCH_VERSION-emergency"
    
    if docker build -t tickstorm:$EMERGENCY_TAG .; then
        echo -e "${GREEN}✅ Emergency patch image built successfully${NC}"
        NEW_IMAGE="tickstorm:$EMERGENCY_TAG"
    else
        echo -e "${RED}❌ Failed to build emergency patch image${NC}"
        exit 1
    fi
    echo ""
fi

# Log deployment start
log_emergency "START" "Deploying to $ENVIRONMENT - $EMERGENCY_REASON"

# Deploy to staging first (if production deployment)
if [[ "$ENVIRONMENT" == "production" ]]; then
    echo -e "${BLUE}🚀 Deploying to staging for validation...${NC}"
    echo "-------------------------------------------"
    
    # Deploy to staging
    kubectl set image deployment/$DEPLOYMENT_NAME-staging tickstorm=$NEW_IMAGE -n $NAMESPACE
    
    # Wait for staging rollout
    if kubectl rollout status deployment/$DEPLOYMENT_NAME-staging -n $NAMESPACE --timeout=${TIMEOUT}s; then
        echo -e "${GREEN}✅ Staging deployment successful${NC}"
    else
        echo -e "${RED}❌ Staging deployment failed${NC}"
        log_emergency "FAILED" "Staging deployment failed"
        exit 1
    fi
    
    # Run staging smoke tests
    echo -e "${BLUE}🧪 Running staging smoke tests...${NC}"
    
    # Get staging service endpoint
    STAGING_ENDPOINT=$(kubectl get service $DEPLOYMENT_NAME-staging -n $NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "localhost")
    STAGING_PORT=$(kubectl get service $DEPLOYMENT_NAME-staging -n $NAMESPACE -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "8080")
    
    # Basic health check
    if timeout 30 bash -c "until curl -f http://$STAGING_ENDPOINT:$STAGING_PORT/health &>/dev/null; do sleep 1; done"; then
        echo -e "${GREEN}✅ Staging health check passed${NC}"
    else
        echo -e "${RED}❌ Staging health check failed${NC}"
        log_emergency "FAILED" "Staging health check failed"
        
        # Rollback staging
        kubectl rollout undo deployment/$DEPLOYMENT_NAME-staging -n $NAMESPACE
        exit 1
    fi
    
    echo ""
fi

# Deploy to target environment
echo -e "${BLUE}🚀 Deploying emergency patch to $ENVIRONMENT...${NC}"
echo "================================================"

# Record deployment start time
DEPLOYMENT_START=$(date +%s)

# Deploy the patch
kubectl set image deployment/$DEPLOYMENT_NAME tickstorm=$NEW_IMAGE -n $NAMESPACE

# Wait for rollout with timeout
echo -e "${BLUE}⏳ Waiting for deployment rollout...${NC}"
if kubectl rollout status deployment/$DEPLOYMENT_NAME -n $NAMESPACE --timeout=${TIMEOUT}s; then
    echo -e "${GREEN}✅ Deployment rollout completed${NC}"
else
    echo -e "${RED}❌ Deployment rollout failed or timed out${NC}"
    log_emergency "FAILED" "Deployment rollout failed"
    
    # Automatic rollback on failure
    echo -e "${YELLOW}🔄 Initiating automatic rollback...${NC}"
    kubectl rollout undo deployment/$DEPLOYMENT_NAME -n $NAMESPACE
    kubectl rollout status deployment/$DEPLOYMENT_NAME -n $NAMESPACE --timeout=300s
    
    log_emergency "ROLLBACK" "Automatic rollback completed due to deployment failure"
    exit 1
fi

# Post-deployment validation
echo -e "${BLUE}🔍 Running post-deployment validation...${NC}"
echo "---------------------------------------"

# Get service endpoint
SERVICE_ENDPOINT=$(kubectl get service $DEPLOYMENT_NAME -n $NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "localhost")
SERVICE_PORT=$(kubectl get service $DEPLOYMENT_NAME -n $NAMESPACE -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "8080")

# Health check with retry
HEALTH_CHECK_ATTEMPTS=0
MAX_HEALTH_ATTEMPTS=30

while [[ $HEALTH_CHECK_ATTEMPTS -lt $MAX_HEALTH_ATTEMPTS ]]; do
    if curl -f http://$SERVICE_ENDPOINT:$SERVICE_PORT/health &>/dev/null; then
        echo -e "${GREEN}✅ Health check passed${NC}"
        break
    fi
    
    HEALTH_CHECK_ATTEMPTS=$((HEALTH_CHECK_ATTEMPTS + 1))
    echo -e "${YELLOW}⏳ Health check attempt $HEALTH_CHECK_ATTEMPTS/$MAX_HEALTH_ATTEMPTS...${NC}"
    sleep 2
done

if [[ $HEALTH_CHECK_ATTEMPTS -eq $MAX_HEALTH_ATTEMPTS ]]; then
    echo -e "${RED}❌ Health check failed after $MAX_HEALTH_ATTEMPTS attempts${NC}"
    log_emergency "FAILED" "Post-deployment health check failed"
    
    # Automatic rollback
    echo -e "${YELLOW}🔄 Initiating automatic rollback due to health check failure...${NC}"
    kubectl rollout undo deployment/$DEPLOYMENT_NAME -n $NAMESPACE
    kubectl rollout status deployment/$DEPLOYMENT_NAME -n $NAMESPACE --timeout=300s
    
    log_emergency "ROLLBACK" "Automatic rollback completed due to health check failure"
    exit 1
fi

# Performance validation
echo -e "${BLUE}📊 Running performance validation...${NC}"

# Basic performance test (response time)
RESPONSE_TIME=$(curl -o /dev/null -s -w '%{time_total}' http://$SERVICE_ENDPOINT:$SERVICE_PORT/health 2>/dev/null || echo "999")
RESPONSE_TIME_MS=$(echo "$RESPONSE_TIME * 1000" | bc 2>/dev/null || echo "999")

if (( $(echo "$RESPONSE_TIME_MS < 1000" | bc -l) )); then
    echo -e "${GREEN}✅ Performance validation passed (${RESPONSE_TIME_MS}ms)${NC}"
else
    echo -e "${YELLOW}⚠️  Performance validation warning (${RESPONSE_TIME_MS}ms > 1000ms)${NC}"
fi

# Calculate deployment time
DEPLOYMENT_END=$(date +%s)
DEPLOYMENT_DURATION=$((DEPLOYMENT_END - DEPLOYMENT_START))

# Success summary
echo ""
echo -e "${GREEN}🎉 EMERGENCY PATCH DEPLOYMENT SUCCESSFUL 🎉${NC}"
echo "================================================"
echo -e "${BLUE}Patch Version:${NC} $PATCH_VERSION"
echo -e "${BLUE}Environment:${NC} $ENVIRONMENT"
echo -e "${BLUE}Previous Image:${NC} $CURRENT_IMAGE"
echo -e "${BLUE}New Image:${NC} $NEW_IMAGE"
echo -e "${BLUE}Deployment Duration:${NC} ${DEPLOYMENT_DURATION}s"
echo -e "${BLUE}Health Check:${NC} ✅ Passed"
echo -e "${BLUE}Response Time:${NC} ${RESPONSE_TIME_MS}ms"
echo -e "${BLUE}Completed:${NC} $(date)"

# Log successful deployment
log_emergency "SUCCESS" "Deployment completed in ${DEPLOYMENT_DURATION}s"

# Post-deployment monitoring recommendations
echo ""
echo -e "${BLUE}📊 Post-Deployment Monitoring Recommendations:${NC}"
echo "1. Monitor error rates for next 30 minutes"
echo "2. Watch performance metrics for degradation"
echo "3. Check logs for any new error patterns"
echo "4. Verify security patch effectiveness"
echo "5. Notify stakeholders of successful deployment"

# Generate deployment report
cat > emergency-deployment-report.txt << EOF
Emergency Security Patch Deployment Report
==========================================

Deployment Details:
- Patch Version: $PATCH_VERSION
- Environment: $ENVIRONMENT
- Reason: $EMERGENCY_REASON
- Timestamp: $(date)
- Duration: ${DEPLOYMENT_DURATION} seconds

Images:
- Previous: $CURRENT_IMAGE
- New: $NEW_IMAGE

Validation Results:
- Health Check: PASSED
- Response Time: ${RESPONSE_TIME_MS}ms
- Deployment Status: SUCCESS

Next Steps:
1. Monitor system for 30 minutes
2. Verify security patch effectiveness
3. Update security documentation
4. Notify security team of completion
EOF

echo -e "${BLUE}📄 Deployment report saved to: emergency-deployment-report.txt${NC}"
echo ""
echo -e "${GREEN}✅ Emergency security patch deployment completed successfully${NC}"
