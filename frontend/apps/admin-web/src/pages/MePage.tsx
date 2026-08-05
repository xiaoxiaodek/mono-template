import { ReloadOutlined, UserOutlined } from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import { Alert, Avatar, Button, Card, Descriptions, Skeleton, Space, Tag, Typography } from "antd";
import { useEffect } from "react";

import { ApiError, type ControlApiClient } from "@vort-ads/api-client";

interface MePageProps {
  accessToken: string;
  client: ControlApiClient;
  onUnauthorized: () => void;
}

export function MePage({ accessToken, client, onUnauthorized }: MePageProps) {
  const currentUser = useQuery({
    queryKey: ["current-user", accessToken],
    queryFn: () => client.me(accessToken),
    retry: (failureCount, error) =>
      !isUnauthorized(error) && failureCount < 1,
  });

  useEffect(() => {
    if (isUnauthorized(currentUser.error)) {
      onUnauthorized();
    }
  }, [currentUser.error, onUnauthorized]);

  return (
    <Card className="user-card" variant="borderless">
      <div className="panel-header">
        <Space>
          <Avatar icon={<UserOutlined />} />
          <div>
            <Typography.Title level={3}>Current user</Typography.Title>
            <Typography.Text type="secondary">
              Identity attached to this control session
            </Typography.Text>
          </div>
        </Space>
        <Button
          aria-label="Refresh current user"
          icon={<ReloadOutlined />}
          loading={currentUser.isFetching}
          onClick={() => void currentUser.refetch()}
        >
          Refresh
        </Button>
      </div>

      {currentUser.isPending ? <Skeleton active paragraph={{ rows: 3 }} /> : null}
      {currentUser.isError ? (
        <Alert
          message="Unable to load the current user"
          description={currentUser.error.message}
          type="error"
          showIcon
        />
      ) : null}
      {currentUser.data ? (
        <Descriptions bordered column={1} size="middle">
          <Descriptions.Item label="Email">
            {currentUser.data.email}
          </Descriptions.Item>
          <Descriptions.Item label="User ID">
            <Typography.Text code>{currentUser.data.id}</Typography.Text>
          </Descriptions.Item>
          <Descriptions.Item label="Roles">
            <Space wrap>
              {currentUser.data.roles.map((role) => (
                <Tag key={role}>{role}</Tag>
              ))}
            </Space>
          </Descriptions.Item>
        </Descriptions>
      ) : null}
    </Card>
  );
}

function isUnauthorized(error: unknown) {
  return error instanceof ApiError && error.code === "UNAUTHORIZED";
}
