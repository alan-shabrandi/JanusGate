import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "10s", target: 20 },
    { duration: "20s", target: 50 },
    { duration: "10s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(95)<200"],
    http_req_failed: ["rate<0.05"],
  },
};

const BASE_URL = __ENV.GATEWAY_URL || "http://localhost:8080";
const JWT_TOKEN = __ENV.JWT_TOKEN || "YOUR_VALID_TEST_JWT_TOKEN";

export default function () {
  const params = {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${JWT_TOKEN}`,
    },
  };

  const resUsers = http.get(`${BASE_URL}/api/v1/users/profile`, params);

  check(resUsers, {
    "status is 200 or 429": (r) => r.status === 200 || r.status === 429,
    "rate limit handled properly": (r) => r.status !== 500,
  });

  const resPublic = http.get(`${BASE_URL}/api/v1/public/ping`);

  check(resPublic, {
    "public route status is 200": (r) => r.status === 200,
  });

  sleep(0.1);
}
