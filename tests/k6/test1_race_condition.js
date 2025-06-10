import http from 'k6/http';
import { Counter } from 'k6/metrics';

export const successfulPurchases = new Counter('successful_purchases');
export const failedPurchases = new Counter('failed_purchases');

export const options = {
    scenarios: {
        race_condition: {
            executor: 'shared-iterations',
            vus: 700,
            iterations: 700,
            maxDuration: '30s',
        },
    },
};

const BASE_URL = 'http://localhost:8032';
const TARGET_ITEM_ID = 1;

export default function () {
    const userId = `race_user_${__VU}_${__ITER}`;

    // Step 1: Checkout
    const checkoutUrl = `${BASE_URL}/checkout?user_id=${encodeURIComponent(userId)}&id=${TARGET_ITEM_ID}`;
    const checkoutResponse = http.post(checkoutUrl, null, {
        timeout: '10s',
        tags: { name: 'checkout' }
    });

    if (checkoutResponse.status !== 200) {
        failedPurchases.add(1);
        return;
    }

    let checkoutCode;
    try {
        const body = JSON.parse(checkoutResponse.body);
        checkoutCode = body.code;
    } catch (e) {
        failedPurchases.add(1);
        console.log(`${userId} failed to parse checkout response`);
        return;
    }

    // Step 2: Purchase immediately
    const purchaseUrl = `${BASE_URL}/purchase?code=${encodeURIComponent(checkoutCode)}`;
    const purchaseResponse = http.post(purchaseUrl, null, {
        timeout: '10s',
        tags: { name: 'purchase' }
    });

    if (purchaseResponse.status === 200) {
        successfulPurchases.add(1);
    } else {
        failedPurchases.add(1);
    }
}

export function handleSummary(data) {
    const successful = data.metrics.successful_purchases?.values?.count || 0;
    const failed = data.metrics.failed_purchases?.values?.count || 0;

    return {
        'stdout': `
=== TEST 1: RACE CONDITION RESULTS ===
Scenario: 700 users fight for item ID ${TARGET_ITEM_ID}

Results:
✅ Successful: ${successful}
❌ Failed: ${failed}
📊 Total: ${successful + failed}

Expected: 1 success, 699 failures
${successful === 1 && failed === 299 ?
                '🎯 PERFECT! Exactly 1 winner, 699 losers' :
                successful === 1 ?
                    '✅ Good! Only 1 winner (some users may have failed earlier)' :
                    successful > 1 ?
                        '🚨 BUG: Multiple users bought the same item!' :
                        '⚠️  No winners - check if item is available'
            }
=====================================
`,
    };
}