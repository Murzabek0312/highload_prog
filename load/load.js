import http from 'k6/http';
import { check } from 'k6';

const appURL = (__ENV.APP_URL || '').replace(/\/+$/, '');
const targetRPS = Number(__ENV.TARGET_RPS || '50');
const errorRate = Number(__ENV.ERROR_RATE || '0.1');

if (!appURL) {
  throw new Error('APP_URL is required');
}
if (!Number.isInteger(targetRPS) || targetRPS <= 0) {
  throw new Error('TARGET_RPS must be a positive integer');
}
if (!Number.isFinite(errorRate) || errorRate < 0 || errorRate > 1) {
  throw new Error('ERROR_RATE must be between 0 and 1');
}

export const options = {
  scenarios: {
    products: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 20,
      maxVUs: 100,
      stages: [
        { target: targetRPS, duration: '1m' },
        { target: targetRPS, duration: '5m' },
        { target: 0, duration: '30s' },
      ],
      gracefulStop: '10s',
    },
  },
};

const expectedBadRequest = http.expectedStatuses(400);
const methods = ['POST', 'GET', 'PUT', 'DELETE'];

// Each VU owns a separate product and performs one CRUD step per iteration.
let phase = 0;
let productID = null;

export default function () {
  if (Math.random() < errorRate) {
    const response = http.get(`${appURL}/products/not-a-number`, {
      tags: { name: 'GET /products/{id} (expected 400)' },
      responseCallback: expectedBadRequest,
      redirects: 0,
      timeout: '5s',
    });
    check(response, { 'invalid ID returns 400': (r) => r.status === 400 });
    return;
  }

  const method = methods[phase];
  const path = method === 'POST' ? '/products' : `/products/${productID}`;
  const body = method === 'POST' || method === 'PUT'
    ? JSON.stringify({ name: `k6-product-${__VU}`, price: method === 'POST' ? 100 : 200 })
    : null;
  const response = http.request(method, appURL + path, body, {
    headers: { 'Content-Type': 'application/json' },
    tags: { name: `${method} ${method === 'POST' ? '/products' : '/products/{id}'}` },
    redirects: 0,
    timeout: '5s',
  });
  const expectedStatus = method === 'POST' ? 201 : 200;
  if (!check(response, { [`${method} succeeds`]: (r) => r.status === expectedStatus })) {
    return;
  }

  if (method === 'POST') {
    const id = response.json();
    if (!check(id, { 'create returns product ID': (value) => Number.isSafeInteger(value) && value > 0 })) {
      return;
    }
    productID = id;
  } else if (method === 'PUT' || method === 'DELETE') {
    if (!check(response, { [`${method} affects one product`]: (r) => r.json() === 1 })) {
      return;
    }
  }

  if (method === 'DELETE') {
    productID = null;
  }
  phase = (phase + 1) % methods.length;
}
