import { type PropsWithChildren, type ReactNode } from "react";

export const ACCESS_TOKEN_KEY = "vort_ads_access_token";

type ProtectedRouteProps = PropsWithChildren<{
  accessToken: string | null;
  fallback: ReactNode;
}>;

export function ProtectedRoute({
  accessToken,
  children,
  fallback,
}: ProtectedRouteProps) {
  return accessToken ? children : fallback;
}
