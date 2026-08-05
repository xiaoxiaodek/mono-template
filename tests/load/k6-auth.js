import http from "k6/http";
import { check } from "k6";

function boundedInteger(raw, fallback, minimum, maximum) {
  const parsed = Number.parseInt(raw ?? "", 10);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return Math.min(maximum, Math.max(minimum, parsed));
}

const baseURL = (__ENV.BASE_URL || "http://localhost:8080").replace(/\/+$/, "");
const password = __ENV.PASSWORD || "load-test-password-123";
const vus = boundedInteger(__ENV.VUS, 5, 1, 50);
const iterations = boundedInteger(__ENV.ITERATIONS, 3, 1, 20);

export const options = {
  scenarios: {
    auth_flow: {
      executor: "per-vu-iterations",
      vus,
      iterations,
      maxDuration: "2m",
      gracefulStop: "5s",
    },
  },
  thresholds: {
    checks: ["rate>0.99"],
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500", "p(99)<1000"],
  },
};

function tokenPresent(response) {
  try {
    return typeof response.json("data.access_token") === "string";
  } catch (_) {
    return false;
  }
}

export default function () {
  const email = `k6-${Date.now()}-${__VU}-${__ITER}@example.test`;
  const params = {
    headers: { "Content-Type": "application/json" },
    tags: { flow: "auth", endpoint: "register" },
    timeout: "5s",
  };

  const registration = http.post(
    `${baseURL}/api/v1/auth/register`,
    JSON.stringify({ email, password }),
    params,
  );
  const registered = check(registration, {
    "register returns 201": (response) => response.status === 201,
    "register returns access token": tokenPresent,
  });

  if (!registered) {
    return;
  }

  const login = http.post(
    `${baseURL}/api/v1/auth/login`,
    JSON.stringify({ email, password }),
    {
      ...params,
      tags: { flow: "auth", endpoint: "login" },
    },
  );
  check(login, {
    "login returns 200": (response) => response.status === 200,
    "login returns access token": tokenPresent,
  });
}
