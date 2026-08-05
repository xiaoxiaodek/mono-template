import { z } from "zod";
import {
  authDataSchema,
  envelopeSchema,
  successEnvelopeSchema,
  tokenPairSchema,
  userSchema,
  type Envelope,
} from "./schemas";

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly requestId?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export function unwrapEnvelope(envelope: unknown): unknown;
export function unwrapEnvelope<T>(envelope: unknown, dataSchema: z.ZodType<T>): T;
export function unwrapEnvelope<T>(
  envelope: unknown,
  dataSchema?: z.ZodType<T>,
): T | unknown {
  const parsed = envelopeSchema.parse(envelope);
  if (parsed.code !== "OK") {
    throw new ApiError(parsed.code, parsed.message, parsed.request_id);
  }
  return dataSchema ? dataSchema.parse(parsed.data) : parsed.data;
}

export class ControlApiClient {
  constructor(
    baseUrl: string,
    private readonly fetcher: typeof fetch = (input, init) =>
      fetch(input, init),
  ) {
    this.baseUrl = baseUrl.replace(/\/+$/, "");
  }

  private readonly baseUrl: string;

  async login(input: { email: string; password: string }) {
    return this.post("/api/v1/auth/login", input, authDataSchema);
  }

  async register(input: { email: string; password: string }) {
    return this.post("/api/v1/auth/register", input, authDataSchema);
  }

  async refresh(refreshToken: string) {
    return this.post(
      "/api/v1/auth/refresh",
      { refresh_token: refreshToken },
      tokenPairSchema,
    );
  }

  async me(accessToken: string) {
    return this.get("/api/v1/me", accessToken, userSchema);
  }

  private async post<T>(path: string, body: unknown, dataSchema: z.ZodType<T>) {
    const response = await this.fetcher(this.baseUrl + path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return parseEndpointResponse(response, dataSchema);
  }

  private async get<T>(
    path: string,
    accessToken: string,
    dataSchema: z.ZodType<T>,
  ) {
    const response = await this.fetcher(this.baseUrl + path, {
      headers: { Authorization: `Bearer ${accessToken}` },
    });
    return parseEndpointResponse(response, dataSchema);
  }
}

async function parseEndpointResponse<T>(
  response: Response,
  dataSchema: z.ZodType<T>,
): Promise<T> {
  const raw = await response.json();
  const envelope = envelopeSchema.parse(raw);
  if (envelope.code !== "OK") {
    throw new ApiError(envelope.code, envelope.message, envelope.request_id);
  }
  const success = successEnvelopeSchema(z.unknown()).parse(raw);
  return dataSchema.parse(success.data);
}

export {
  authDataSchema,
  envelopeSchema,
  tokenPairSchema,
  userSchema,
  type AuthData,
  type Envelope,
  type TokenPair,
  type User,
} from "./schemas";
