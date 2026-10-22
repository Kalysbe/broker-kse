import { z } from 'zod';

export const decimalSchema = z.string().regex(/^-?\d+(?:\.\d+)?$/, 'Invalid decimal');
export const uuidSchema = z.string().regex(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i);
const uuid = uuidSchema;
export const tokenPairSchema = z.object({ access_token: z.string().min(1), refresh_token: z.string().min(1), expires_in: z.number().positive(), token_type: z.string() });
export const storedSessionSchema = z.object({ server: z.string(), accessToken: z.string().min(1), refreshToken: z.string().min(1), expiresAt: z.number() });
export type StoredSession = z.infer<typeof storedSessionSchema>;
export const userSchema = z.object({ id: uuid, login: z.string(), first_name: z.string(), last_name: z.string(), email: z.string().optional(), phone: z.string().nullable().optional(), client_id: uuid.nullable().optional(), roles: z.array(z.string()).nullable().transform(v => v ?? []), permissions: z.array(z.string()).nullable().transform(v => v ?? []) });
export type User = z.infer<typeof userSchema>;
export const accountSchema = z.object({ id: uuid, account_number: z.string(), client_id: uuid, currency: z.string(), status: z.string() });
export type Account = z.infer<typeof accountSchema>;
export const cashSchema = z.object({ account_id: uuid, buckets: z.record(z.string(), decimalSchema) });
export const positionSchema = z.object({ instrument_id: uuid, ticker: z.string(), quantity: decimalSchema, average_price: decimalSchema.nullable(), market_price: decimalSchema.optional(), market_value: decimalSchema.nullable(), unrealized_pnl: decimalSchema.optional() });
export const portfolioSchema = z.object({ account_id: uuid, available_funds: decimalSchema.nullable(), securities_value: decimalSchema.nullable(), cash: z.record(z.string(), decimalSchema), positions: z.array(positionSchema), source: z.string().optional(), taken_at: z.string().nullable().optional(), cash_reported: z.boolean().optional() });
export const instrumentSchema = z.object({ id: uuid, ticker: z.string(), isin: z.string(), name: z.string(), type: z.string(), currency: z.string(), lot_size: decimalSchema, tick_size: decimalSchema, status: z.string(), board: z.string().optional() });
export const quoteSchema = z.object({ id: uuid, ticker: z.string(), name: z.string(), currency: z.string(), status: z.string(), security_type: z.string(), last: decimalSchema.optional(), bid: decimalSchema.optional(), ask: decimalSchema.optional(), source: z.string(), venue_symbol: z.string().optional() });
export type Quote = z.infer<typeof quoteSchema>;
export type Instrument = z.infer<typeof instrumentSchema>;
export const orderSchema = z.object({ id: z.string().min(1), client_order_id: z.string(), account_id: z.string().min(1), instrument_id: z.string().min(1), symbol: z.string().optional(), board: z.string().optional(), side: z.enum(['BUY', 'SELL']), order_type: z.string(), quantity: decimalSchema, filled_quantity: decimalSchema, price: decimalSchema.nullable().optional(), status: z.string(), created_at: z.string(), can_cancel: z.boolean(), reject_reason: z.string().nullable().optional() });
export type Order = z.infer<typeof orderSchema>;
export const ledgerSchema = z.object({ id: uuid, entry_type: z.string(), debit: decimalSchema, credit: decimalSchema, bucket: z.string(), created_at: z.string() });
export const notificationSchema = z.object({ id: uuid, severity: z.string(), title: z.string(), body: z.string(), created_at: z.string(), read_at: z.string().nullable().optional() });
export function listSchema<T extends z.ZodType>(item: T) { return z.object({ items: z.array(item) }); }


export const connectionSchema = z.object({ connected: z.boolean(), health: z.string(), environment: z.string().optional(), read_only: z.boolean().optional(), realtime_connected: z.boolean().optional() });
