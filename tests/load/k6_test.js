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
    http_req_failed: ["rate<0.95"],
  },
};

const BASE_URL = __ENV.GATEWAY_URL || "http://localhost:8080";

export default function () {
  const params = {
    headers: {
      "Content-Type": "application/json",
    },
  };

  const resUsers = http.get(`${BASE_URL}/api/v1/users`, params);

  check(resUsers, {
    "users status is 200 or 429": (r) => r.status === 200 || r.status === 429,
    "no internal server error (500)": (r) => r.status !== 500,
  });

  const resPayments = http.get(`${BASE_URL}/api/v1/payments`, params);

  check(resPayments, {
    "payments status is 200 or 429": (r) =>
      r.status === 200 || r.status === 429,
  });

  sleep(0.1);
}
