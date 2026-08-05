import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConfigProvider } from "antd";
import { type PropsWithChildren, useState } from "react";

export function AppProviders({ children }: PropsWithChildren) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            refetchOnWindowFocus: false,
            retry: 1,
            staleTime: 30_000,
          },
        },
      }),
  );

  return (
    <ConfigProvider
      theme={{
        token: {
          borderRadius: 6,
          colorPrimary: "#176b87",
          fontFamily:
            'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
        },
      }}
    >
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </ConfigProvider>
  );
}
