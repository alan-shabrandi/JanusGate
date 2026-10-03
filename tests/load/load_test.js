import http from "k6/http";
import { check } from "k6";

export const options = {
  scenarios: {
    gateway_stress_test: {
      executor: "constant-arrival-rate",
      rate: 1000,
      timeUnit: "1s",
      duration: "30s",
      preAllocatedVUs: 100,
      maxVUs: 1000,
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<50", "p(99)<100"],
  },
};

export default function () {
  const url = "http://localhost:8080/api/v1/users/profile";

  const params = {
    headers: {
      "Content-Type": "application/json",
      "X-Request-ID": "k6-load-test-runner",
    },
    timeout: "5s",
  };

  const res = http.get(url, params);

  check(res, {
    "status is 200": (r) => r.status === 200,
  });
}
