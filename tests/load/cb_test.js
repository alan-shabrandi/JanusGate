import http from "k6/http";
import { check, sleep } from "k6";
import { Counter } from "k6/metrics";

const status200 = new Counter("status_200");
const status429 = new Counter("status_429");
const status502 = new Counter("status_502");
const status503 = new Counter("status_503");

export const options = {
  vus: 10,
  duration: "30s",
};

export default function () {
  const res = http.get("http://localhost:8080/api/v1/users");

  if (res.status === 200) status200.add(1);
  else if (res.status === 429) status429.add(1);
  else if (res.status === 502) status502.add(1);
  else if (res.status === 503) status503.add(1);

  check(res, {
    "is valid response (200, 429, 502, 503)": (r) =>
      [200, 429, 502, 503].includes(r.status),
  });

  sleep(0.1);
}
