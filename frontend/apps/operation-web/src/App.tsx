import { LogoutOutlined } from "@ant-design/icons";
import { ControlApiClient, type AuthData } from "@vort-ads/api-client";
import { useQueryClient } from "@tanstack/react-query";
import { Button, Layout, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";

import { AppProviders } from "./app/providers";
import { LoginPage } from "./pages/LoginPage";
import { MePage } from "./pages/MePage";
import { ProtectedRoute } from "./routes/ProtectedRoute";
import {
  readAccessToken,
  removeAccessToken,
  removeLegacyAuthStorage,
  storeAccessToken,
} from "./routes/authStorage";

const apiBaseUrl = import.meta.env.VITE_OPERATION_API_BASE_URL ?? "http://localhost:8080";

function AdminApp() {
  const client = useMemo(() => new ControlApiClient(apiBaseUrl), []);
  const queryClient = useQueryClient();
  const [accessToken, setAccessToken] = useState<string | null>(readAccessToken);

  useEffect(() => {
    removeLegacyAuthStorage();
  }, []);

  function completeLogin(auth: AuthData) {
    storeAccessToken(auth.access_token);
    setAccessToken(auth.access_token);
  }

  const signOut = useCallback(() => {
    removeAccessToken();
    setAccessToken(null);
    queryClient.removeQueries({ queryKey: ["current-user"] });
  }, [queryClient]);

  const login = <LoginPage client={client} onAuthenticated={completeLogin} />;

  return (
    <ProtectedRoute accessToken={accessToken} fallback={login}>
      {accessToken ? (
        <Layout className="app-shell">
          <Layout.Header className="app-header">
            <div className="brand-lockup">
              <span className="brand-mark" aria-hidden>
                V
              </span>
              <div>
                <Typography.Text className="brand-name">Vort Ads</Typography.Text>
                <Typography.Text className="brand-context">
                  Control center
                </Typography.Text>
              </div>
            </div>
            <Button icon={<LogoutOutlined />} onClick={signOut}>
              Sign out
            </Button>
          </Layout.Header>
          <Layout.Content className="app-content">
            <div className="page-heading">
              <Typography.Text className="eyebrow">ACCOUNT</Typography.Text>
              <Typography.Title level={1}>Session overview</Typography.Title>
            </div>
            <MePage
              accessToken={accessToken}
              client={client}
              onUnauthorized={signOut}
            />
          </Layout.Content>
        </Layout>
      ) : (
        login
      )}
    </ProtectedRoute>
  );
}

export default function App() {
  return (
    <AppProviders>
      <AdminApp />
    </AppProviders>
  );
}
