import { ACCESS_TOKEN_KEY } from "./ProtectedRoute";

const legacyRefreshTokenKey = "vort_ads_refresh_token";

function safely(operation: () => void) {
  try {
    operation();
  } catch {
    // Browsers can deny Web Storage access. Authentication still works in memory.
  }
}

export function readAccessToken() {
  try {
    return sessionStorage.getItem(ACCESS_TOKEN_KEY);
  } catch {
    return null;
  }
}

export function storeAccessToken(accessToken: string) {
  safely(() => sessionStorage.setItem(ACCESS_TOKEN_KEY, accessToken));
}

export function removeAccessToken() {
  safely(() => sessionStorage.removeItem(ACCESS_TOKEN_KEY));
}

export function removeLegacyAuthStorage() {
  safely(() => localStorage.removeItem(legacyRefreshTokenKey));
  safely(() => sessionStorage.removeItem(legacyRefreshTokenKey));
  safely(() => localStorage.removeItem(ACCESS_TOKEN_KEY));
}
