import http from 'k6/http';
import { Counter } from 'k6/metrics';

export const successfulPurchases = new Counter('successful_purchases');
export const limitBlocked = new Counter('limit_blocked');
export const userSuccessCount = new Counter('user_success_count');

export const options = {
    scenarios: {
        user_limits: {
            executor: 'per-vu-iterations',
            vus: 10,
            iterations: 100,
            maxDuration: '5m',
        },
    },
};

const BASE_URL = 'http://localhost:8032';
const ITEM_START_ID = 1;
const MAX_ITEMS_PER_USER = 10;

export default function () {
    const userId = `user_${__VU}`;
    const itemId = ITEM_START_ID + (__VU - 1) * 100 + __ITER;
    const attemptNumber = __ITER + 1;

    console.log(`${userId} attempt ${attemptNumber}: trying to buy item ${itemId}`);

    // Step 1: Checkout
    const checkoutUrl = `${BASE_URL}/checkout?user_id=${encodeURIComponent(userId)}&id=${itemId}`;
    const checkoutResponse = http.post(checkoutUrl, null, {
        timeout: '10s',
        tags: { name: 'checkout' }
    });

    if (checkoutResponse.status !== 200) {
        if (checkoutResponse.status === 403) {
            limitBlocked.add(1);
            console.log(`🚫 ${userId} blocked at checkout attempt ${attemptNumber}`);
        }
        return;
    }

    let checkoutCode;
    try {
        const body = JSON.parse(checkoutResponse.body);
        checkoutCode = body.code;
    } catch (e) {
        console.log(`${userId} failed to parse checkout response`);
        return;
    }

    const purchaseUrl = `${BASE_URL}/purchase?code=${encodeURIComponent(checkoutCode)}`;
    const purchaseResponse = http.post(purchaseUrl, null, {
        timeout: '10s',
        tags: { name: 'purchase' }
    });

    if (purchaseResponse.status === 200) {
        successfulPurchases.add(1);
        userSuccessCount.add(1);
        console.log(`✅ ${userId} purchase ${attemptNumber} successful!`);
    } else if (purchaseResponse.status === 409) {
        limitBlocked.add(1);
        console.log(`🚫 ${userId} blocked at purchase attempt ${attemptNumber}`);
    }
}

export function handleSummary(data) {
    const successful = data.metrics.successful_purchases?.values?.count || 0;
    const blocked = data.metrics.limit_blocked?.values?.count || 0;
    const expectedSuccessful = 10 * MAX_ITEMS_PER_USER;

    return {
        'stdout': `
=== TEST 2: USER LIMITS RESULTS ===
Scenario: 10 users each try to buy 100 items (limit: ${MAX_ITEMS_PER_USER} per user)

Results:
✅ Successful Purchases: ${successful}
🚫 Blocked by Limit: ${blocked}
📊 Expected Successful: ${expectedSuccessful}

Per User Analysis:
Expected: Each user buys exactly ${MAX_ITEMS_PER_USER} items
${successful === expectedSuccessful ?
                '🎯 PERFECT! All users limited to exactly 10 items each' :
                successful <= expectedSuccessful ?
                    '✅ User limits working (some users may have stopped early)' :
                    '🚨 BUG: Users bought more than allowed!'
            }
=====================================
`,
    };
}