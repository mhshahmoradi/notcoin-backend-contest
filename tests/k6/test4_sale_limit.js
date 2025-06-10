import http from 'k6/http';
import { Counter, Rate } from 'k6/metrics';

export const successfulPurchases = new Counter('successful_purchases');
export const saleLimitBlocked = new Counter('sale_limit_blocked');
export const otherFailures = new Counter('other_failures');
export const noOverSelling = new Rate('no_over_selling');

export const options = {
    scenarios: {
        heavy_load: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '0s', target: 200 },
                { duration: '30s', target: 200 },
                { duration: '30s', target: 0 },
            ],
        },
    },
    thresholds: {
        successful_purchases: ['count<=10000'],
        no_over_selling: ['rate>0.99'],
    },
};

const BASE_URL = 'http://localhost:8032';
const ITEM_START_ID = 1;
const ITEM_END_ID = 10000;
const MAX_SALE_ITEMS = 10000;

export default function () {
    const userId = `heavy_user_${__VU}_${__ITER}_${Date.now()}`;
    const itemId = Math.floor(Math.random() * (ITEM_END_ID - ITEM_START_ID + 1)) + ITEM_START_ID;

    const checkoutUrl = `${BASE_URL}/checkout?user_id=${encodeURIComponent(userId)}&id=${itemId}`;
    const checkoutResponse = http.post(checkoutUrl, null, {
        timeout: '15s',
        tags: { name: 'checkout' }
    });

    if (checkoutResponse.status !== 200) {
        if (checkoutResponse.status === 409) {
            saleLimitBlocked.add(1);
            if (Math.random() < 0.01) {
                console.log(`🚫 Sale limit reached - user ${userId}`);
            }
        } else {
            otherFailures.add(1);
        }
        return;
    }

    let checkoutCode;
    try {
        const body = JSON.parse(checkoutResponse.body);
        checkoutCode = body.code;
    } catch (e) {
        otherFailures.add(1);
        return;
    }

    // Step 2: Purchase
    const purchaseUrl = `${BASE_URL}/purchase?code=${encodeURIComponent(checkoutCode)}`;
    const purchaseResponse = http.post(purchaseUrl, null, {
        timeout: '15s',
        tags: { name: 'purchase' }
    });

    if (purchaseResponse.status === 200) {
        const currentCount = successfulPurchases.add(1);

        if (currentCount <= MAX_SALE_ITEMS) {
            noOverSelling.add(true);
            if (currentCount % 1000 === 0) {
                console.log(`📊 Progress: ${currentCount} items sold`);
            }
        } else {
            noOverSelling.add(false);
            console.log(`🚨 OVER-SELLING DETECTED! Count: ${currentCount}`);
        }
    } else if (purchaseResponse.status === 409) {
        saleLimitBlocked.add(1);
        noOverSelling.add(true);
    } else {
        otherFailures.add(1);
    }
}

export function handleSummary(data) {
    const successful = data.metrics.successful_purchases?.values?.count || 0;
    const blocked = data.metrics.sale_limit_blocked?.values?.count || 0;
    const failures = data.metrics.other_failures?.values?.count || 0;
    const totalAttempts = successful + blocked + failures;

    return {
        'stdout': `
=== TEST 3: SALE LIMIT RESULTS ===
Scenario: Heavy load test with max 10,000 total items

Results:
✅ Successful Purchases: ${successful}
🚫 Blocked by Sale Limit: ${blocked}
❌ Other Failures: ${failures}
📊 Total Attempts: ${totalAttempts}

Sale Limit Analysis:
Maximum allowed: ${MAX_SALE_ITEMS}
${successful <= MAX_SALE_ITEMS ?
                successful === MAX_SALE_ITEMS ?
                    '🎯 PERFECT! Exactly 10,000 items sold' :
                    '✅ No over-selling detected' :
                '🚨 CRITICAL BUG: Over-selling detected!'
            }

Items tested: ${ITEM_START_ID} to ${ITEM_END_ID}
Success rate: ${totalAttempts > 0 ? (successful / totalAttempts * 100).toFixed(2) : 0}%

${successful > MAX_SALE_ITEMS ?
                'IMMEDIATE ACTION REQUIRED: Your system sold more items than available!' :
                'System correctly enforced the 10,000 item limit'
            }
=====================================
`,
    };
}