import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  scenarios: {
    api: { executor: "constant-arrival-rate", rate: 50, timeUnit: "1s", duration: "2m", preAllocatedVUs: 20 },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500"],
  },
};

const BASE = __ENV.BASE_URL || "http://localhost:18080";

export function setup() {
  const res = http.post(`${BASE}/v1/auth/login`, JSON.stringify({
    login: __ENV.LOGIN || "admin",
    password: __ENV.PASSWORD || "ChangeMe_Admin12",
  }), { headers: { "Content-Type": "application/json" } });
  return { token: res.json("access_token") };
}

export default function (data) {
  const h = { Authorization: `Bearer ${data.token}` };
  check(http.get(`${BASE}/health/ready`), { ready: (r) => r.status === 200 });
  check(http.get(`${BASE}/v1/dashboard`, { headers: h }), { dash: (r) => r.status === 200 });
  check(http.get(`${BASE}/v1/orders`, { headers: h }), { orders: (r) => r.status === 200 });
  sleep(0.1);
}
