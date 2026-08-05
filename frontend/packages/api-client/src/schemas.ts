import { z } from "zod";

export const envelopeSchema = z.object({
  code: z.string(),
  message: z.string(),
  request_id: z.string().optional(),
  data: z.unknown().optional(),
});

export const userSchema = z.object({
  id: z.string().min(1),
  email: z.string().email(),
  roles: z.array(z.string()),
  created_at: z.string().datetime().optional(),
  updated_at: z.string().datetime().optional(),
});

export const tokenPairSchema = z.object({
  access_token: z.string().min(1),
  refresh_token: z.string().min(1),
  token_type: z.literal("Bearer").optional(),
  expires_in: z.number().int().positive().optional(),
});

export const authDataSchema = tokenPairSchema.extend({
  user: userSchema,
});

export function successEnvelopeSchema<T extends z.ZodTypeAny>(dataSchema: T) {
  return z.object({
    code: z.literal("OK"),
    message: z.string(),
    request_id: z.string().min(1),
    data: dataSchema,
  });
}

export type Envelope<T = unknown> = {
  code: string;
  message: string;
  request_id?: string;
  data?: T;
};

export type User = z.infer<typeof userSchema>;
export type TokenPair = z.infer<typeof tokenPairSchema>;
export type AuthData = z.infer<typeof authDataSchema>;
