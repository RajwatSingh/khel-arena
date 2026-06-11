// Service-role Supabase client.
// ONLY for trusted server contexts: payment intents and gateway callbacks,
// where there is no user session (the request comes from eSewa/Khalti's
// redirect) but the database must still be updated. Never import this in
// client components — the service key bypasses RLS.
import { createClient as createSupabaseClient } from "@supabase/supabase-js";

export function createAdminClient() {
  return createSupabaseClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.SUPABASE_SERVICE_ROLE_KEY!,
    { auth: { persistSession: false, autoRefreshToken: false } }
  );
}

export const isPaymentsConfigured = () =>
  Boolean(process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.SUPABASE_SERVICE_ROLE_KEY);
