import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import App from "./App";
import { ACCESS_TOKEN_KEY } from "./routes/ProtectedRoute";

const legacyRefreshTokenKey = "vort_ads_refresh_token";

function apiResponse(data: unknown) {
  return Promise.resolve({
    json: () =>
      Promise.resolve({
        code: "OK",
        message: "ok",
        request_id: "req_test",
        data,
      }),
  } as Response);
}

function apiError(code: string, message: string) {
  return Promise.resolve({
    json: () =>
      Promise.resolve({
        code,
        message,
        request_id: "req_test",
      }),
  } as Response);
}

describe("App", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders login form by default", () => {
    render(<App />);

    expect(
      screen.getByRole("button", { name: /sign in/i }),
    ).toBeInTheDocument();
  });

  it("stores only the access token in session storage after login", async () => {
    localStorage.setItem(legacyRefreshTokenKey, "legacy-local-refresh");
    sessionStorage.setItem(legacyRefreshTokenKey, "legacy-session-refresh");
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementationOnce(() =>
          apiResponse({
            access_token: "short-lived-access",
            refresh_token: "long-lived-refresh",
            user: {
              id: "usr_1",
              email: "admin@example.com",
              roles: ["admin"],
            },
          }),
        )
        .mockImplementationOnce(() =>
          apiResponse({
            id: "usr_1",
            email: "admin@example.com",
            roles: ["admin"],
          }),
        ),
    );

    render(<App />);
    fireEvent.change(screen.getByLabelText(/email/i), {
      target: { value: "admin@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "password123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe(
        "short-lived-access",
      );
    });
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
    expect(localStorage.getItem(legacyRefreshTokenKey)).toBeNull();
    expect(sessionStorage.getItem(legacyRefreshTokenKey)).toBeNull();
    expect(setItem).not.toHaveBeenCalledWith(
      expect.anything(),
      "long-lived-refresh",
    );
  });

  it("clears the session access token on sign out", async () => {
    sessionStorage.setItem(ACCESS_TOKEN_KEY, "short-lived-access");
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        apiResponse({
          id: "usr_1",
          email: "admin@example.com",
          roles: ["admin"],
        }),
      ),
    );

    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: /sign out/i }));

    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
    expect(
      screen.getByRole("button", { name: /sign in/i }),
    ).toBeInTheDocument();
  });

  it("clears an unauthorized session without retrying the current-user request", async () => {
    sessionStorage.setItem(ACCESS_TOKEN_KEY, "expired-access");
    const fetcher = vi.fn(() => apiError("UNAUTHORIZED", "login required"));
    vi.stubGlobal("fetch", fetcher);

    render(<App />);

    expect(
      await screen.findByRole("button", { name: /sign in/i }),
    ).toBeInTheDocument();
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("renders the login page when session storage cannot be read", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("storage blocked", "SecurityError");
    });

    expect(() => render(<App />)).not.toThrow();
    expect(
      screen.getByRole("button", { name: /sign in/i }),
    ).toBeInTheDocument();
  });

  it("signs out in memory when removing the stored token fails", async () => {
    sessionStorage.setItem(ACCESS_TOKEN_KEY, "short-lived-access");
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        apiResponse({
          id: "usr_1",
          email: "admin@example.com",
          roles: ["admin"],
        }),
      ),
    );
    vi.spyOn(Storage.prototype, "removeItem").mockImplementation(function (
      this: Storage,
      key,
    ) {
      if (this === window.sessionStorage && key === ACCESS_TOKEN_KEY) {
        throw new DOMException("storage blocked", "SecurityError");
      }
    });

    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: /sign out/i }));

    expect(
      screen.getByRole("button", { name: /sign in/i }),
    ).toBeInTheDocument();
  });
});
