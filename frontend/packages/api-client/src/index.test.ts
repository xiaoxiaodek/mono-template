import { afterEach, describe, expect, it, vi } from "vitest";
import { ControlApiClient, unwrapEnvelope } from "./index";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("unwrapEnvelope", () => {
  it("returns data for OK envelopes", () => {
    const out = unwrapEnvelope({
      code: "OK",
      message: "ok",
      data: { id: "usr_1" },
    });

    expect(out).toEqual({ id: "usr_1" });
  });

  it("throws for error envelopes", () => {
    expect(() =>
      unwrapEnvelope({ code: "UNAUTHORIZED", message: "login required" }),
    ).toThrow("login required");
  });
});

describe("ControlApiClient", () => {
  it("calls the default fetcher without rebinding its receiver", async () => {
    const receiverSensitiveFetch = vi.fn(function (this: unknown) {
      if (this !== undefined) {
        throw new TypeError("Illegal invocation");
      }

      return Promise.resolve(
        responseWith({
          code: "OK",
          message: "ok",
          request_id: "req_1",
          data: validAuthData,
        }),
      );
    });
    vi.stubGlobal("fetch", receiverSensitiveFetch);
    const client = new ControlApiClient("https://api.example.com");

    await client.login({
      email: "admin@example.com",
      password: "password123",
    });

    expect(receiverSensitiveFetch).toHaveBeenCalledTimes(1);
  });

  it("sends login credentials as JSON", async () => {
    const fetcher = vi.fn(async () =>
      responseWith({
        code: "OK",
        message: "ok",
        request_id: "req_1",
        data: validAuthData,
      }),
    );
    const client = new ControlApiClient("https://api.example.com", fetcher as typeof fetch);

    await client.login({ email: "admin@example.com", password: "password123" });

    expect(fetcher).toHaveBeenCalledWith(
      "https://api.example.com/api/v1/auth/login",
      expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: "admin@example.com",
          password: "password123",
        }),
      }),
    );
  });

  it("normalizes a trailing slash in the base URL", async () => {
    const fetcher = vi.fn(async () =>
      responseWith({
        code: "OK",
        message: "ok",
        request_id: "req_1",
        data: validAuthData,
      }),
    );
    const client = new ControlApiClient("https://api.example.com/", fetcher as typeof fetch);

	await client.login({ email: "admin@example.com", password: "password123" });

	expect(fetcher).toHaveBeenCalledWith(
	  "https://api.example.com/api/v1/auth/login",
	  expect.anything(),
	);
  });

  it("rejects malformed successful endpoint data", async () => {
    const fetcher = vi.fn(async () =>
      responseWith({
        code: "OK",
        message: "ok",
        request_id: "req_1",
        data: { access_token: "missing the rest" },
      }),
    );
    const client = new ControlApiClient("https://api.example.com", fetcher as typeof fetch);

	await expect(
	  client.login({ email: "admin@example.com", password: "password123" }),
	).rejects.toThrow();
  });

  it("sends the bearer token when loading the current user", async () => {
    const fetcher = vi.fn(async () =>
      responseWith({
        code: "OK",
        message: "ok",
        request_id: "req_1",
        data: validAuthData.user,
      }),
    );
    const client = new ControlApiClient("https://api.example.com", fetcher as typeof fetch);

    await client.me("access-token");

    expect(fetcher).toHaveBeenCalledWith(
      "https://api.example.com/api/v1/me",
      expect.objectContaining({
        headers: { Authorization: "Bearer access-token" },
      }),
    );
  });
});

function responseWith(body: unknown): Response {
  return { json: async () => body } as Response;
}

const validAuthData = {
  user: {
    id: "usr_1",
    email: "admin@example.com",
    roles: ["admin"],
  },
  access_token: "access-token",
  refresh_token: "refresh-token",
};
