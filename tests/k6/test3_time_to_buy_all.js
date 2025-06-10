import http from 'k6/http';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import exec from 'k6/execution';

// Metrics
export let checkoutErrors = new Rate('checkout_errors');
export let purchaseErrors = new Rate('purchase_errors');
export let successfulPurchases = new Counter('successful_purchases');
export let failedAttempts = new Counter('failed_attempts');

export let options = {
    vus: 1000,
    iterations: 10000,
    thresholds: {
        successful_purchases: ['count>=10000'],
        checkout_errors: ['rate<0.05'],
        purchase_errors: ['rate<0.05'],
    },
};

const BASE_URL = 'http://localhost:8032';

export default function () {
    const itemID = exec.scenario.iterationInTest + 1;
    const userID = `user_${__VU * itemID}`;

    const checkoutRes = http.post(
        `${BASE_URL}/checkout?user_id=${userID}&id=${itemID}`,
        null,
        {
            headers: { 'Content-Type': 'application/json' },
            timeout: '10s',
            tags: { name: 'checkout' }
        }
    );

    const checkoutOk = check(checkoutRes, {
        'checkout status 200': (r) => r.status === 200,
    });

    if (!checkoutOk) {
        console.log(`CHECKOUT FAILED: Item ID ${itemID}, User ID ${userID}, Status: ${checkoutRes.status}, Response: ${checkoutRes.body}`);
        checkoutErrors.add(1);
        failedAttempts.add(1);
        return;
    }
    checkoutErrors.add(0);

    const checkoutData = checkoutRes.json();
    if (!checkoutData || !checkoutData.code) {
        console.log(`CHECKOUT NO CODE: Item ID ${itemID}, User ID ${userID}, Response: ${checkoutRes.body}`);
        failedAttempts.add(1);
        return;
    }

    const purchaseRes = http.post(
        `${BASE_URL}/purchase?code=${checkoutData.code}`,
        null,
        {
            headers: { 'Content-Type': 'application/json' },
            timeout: '10s',
            tags: { name: 'purchase' }
        }
    );

    const purchaseOk = check(purchaseRes, {
        'purchase status 200': (r) => r.status === 200,
    });

    if (!purchaseOk) {
        console.log(`PURCHASE FAILED: Item ID ${itemID}, User ID ${userID}, Code: ${checkoutData.code}, Status: ${purchaseRes.status}, Response: ${purchaseRes.body}`);
        purchaseErrors.add(1);
        failedAttempts.add(1);
        return;
    }
    purchaseErrors.add(0);

    const purchaseData = purchaseRes.json();
    if (purchaseData && purchaseData.status === 'success') {
        successfulPurchases.add(1);
    } else {
        console.log(`PURCHASE NOT SUCCESS: Item ID ${itemID}, User ID ${userID}, Code: ${checkoutData.code}, Response: ${JSON.stringify(purchaseData)}`);
        failedAttempts.add(1);
    }
}