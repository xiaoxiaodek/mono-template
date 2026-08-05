import { LockOutlined, MailOutlined } from "@ant-design/icons";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Space,
  Typography,
} from "antd";
import { useState } from "react";
import { z } from "zod";

import type { AuthData, ControlApiClient } from "@vort-ads/api-client";

const loginSchema = z.object({
  email: z.string().trim().email("Enter a valid email address"),
  password: z
    .string()
    .min(8, "Password must be at least 8 characters")
    .max(128, "Password must be at most 128 characters"),
});

type LoginInput = z.infer<typeof loginSchema>;

interface LoginPageProps {
  client: ControlApiClient;
  onAuthenticated: (auth: AuthData) => void;
}

export function LoginPage({ client, onAuthenticated }: LoginPageProps) {
  const [form] = Form.useForm<LoginInput>();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>();

  async function submit(values: LoginInput) {
    setError(undefined);
    const parsed = loginSchema.safeParse(values);

    if (!parsed.success) {
      form.setFields(
        parsed.error.issues.map((issue) => ({
          name: issue.path[0] as keyof LoginInput,
          errors: [issue.message],
        })),
      );
      return;
    }

    setSubmitting(true);
    try {
      onAuthenticated(await client.login(parsed.data));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Sign in failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-page">
      <section className="login-intro" aria-labelledby="product-name">
        <Typography.Text className="eyebrow">VORT ADS</Typography.Text>
        <Typography.Title id="product-name" level={1}>
          Control center
        </Typography.Title>
        <Typography.Paragraph>
          Secure operational access for campaign infrastructure and platform
          administration.
        </Typography.Paragraph>
      </section>

      <Card className="login-card" variant="borderless">
        <Space direction="vertical" size={4} className="login-heading">
          <Typography.Title level={2}>Sign in</Typography.Title>
          <Typography.Text type="secondary">
            Use your administrator credentials.
          </Typography.Text>
        </Space>

        {error ? <Alert message={error} type="error" showIcon /> : null}

        <Form<LoginInput>
          form={form}
          layout="vertical"
          requiredMark={false}
          onFinish={submit}
        >
          <Form.Item label="Email" name="email">
            <Input
              autoComplete="email"
              prefix={<MailOutlined aria-hidden />}
              placeholder="admin@example.com"
              size="large"
            />
          </Form.Item>
          <Form.Item label="Password" name="password">
            <Input.Password
              autoComplete="current-password"
              prefix={<LockOutlined aria-hidden />}
              size="large"
            />
          </Form.Item>
          <Button
            block
            htmlType="submit"
            loading={submitting}
            size="large"
            type="primary"
          >
            Sign in
          </Button>
        </Form>
      </Card>
    </main>
  );
}
