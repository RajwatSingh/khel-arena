"use server";

// ============================================================================
// Auth server actions — sign up, sign in, sign out.
// Sessions are cookie-bound through the SSR client (see lib/supabase/server),
// so signing in/out here writes the auth cookies and middleware.ts keeps them
// fresh on every request.
//
// profiles has no INSERT RLS policy (rows are meant to be seeded alongside the
// auth user), so sign-up creates the profile row with the service-role admin
// client — the same trusted server client used by the payment callbacks.
// ============================================================================

import { revalidatePath } from "next/cache";
import { z } from "zod";
import { createClient } from "@/lib/supabase/server";
import { createAdminClient } from "@/lib/supabase/admin";
import type { ActionResult } from "@/lib/types";

const SignUpSchema = z.object({
  fullName: z.string().trim().min(2, "Tell us your name.").max(60),
  username: z
    .string()
    .trim()
    .toLowerCase()
    .regex(/^[a-z0-9_]{3,24}$/, "Username: 3–24 chars — lowercase letters, numbers, or _."),
  email: z.string().trim().email("Enter a valid email."),
  password: z.string().min(8, "Use at least 8 characters."),
});
export type SignUpInput = z.input<typeof SignUpSchema>;

const SignInSchema = z.object({
  email: z.string().trim().email("Enter a valid email."),
  password: z.string().min(1, "Enter your password."),
});
export type SignInInput = z.input<typeof SignInSchema>;

export async function signUp(
  input: SignUpInput
): Promise<ActionResult<{ needsConfirmation: boolean }>> {
  const parsed = SignUpSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }
  const v = parsed.data;

  if (!process.env.SUPABASE_SERVICE_ROLE_KEY) {
    return { ok: false, error: "Sign-up isn't configured yet — set SUPABASE_SERVICE_ROLE_KEY." };
  }

  const admin = createAdminClient();

  // Friendly check before we create the auth user — usernames are unique.
  const { data: taken } = await admin
    .from("profiles")
    .select("id")
    .eq("username", v.username)
    .maybeSingle();
  if (taken) return { ok: false, error: "That username is taken.", code: "USERNAME_TAKEN" };

  const supabase = await createClient();
  const { data, error } = await supabase.auth.signUp({
    email: v.email,
    password: v.password,
  });
  if (error || !data.user) {
    return { ok: false, error: error?.message ?? "Could not create your account." };
  }

  // Supabase doesn't error on a re-used email (anti-enumeration): it returns an
  // obfuscated user with an empty identities array. Detect that and say so,
  // rather than trying to seed a profile against a non-existent auth row.
  if (data.user.identities && data.user.identities.length === 0) {
    return {
      ok: false,
      error: "An account with this email already exists. Try signing in instead.",
      code: "EMAIL_TAKEN",
    };
  }

  // Seed the profile row (no INSERT policy → service role).
  const { error: profileError } = await admin.from("profiles").insert({
    id: data.user.id,
    username: v.username,
    full_name: v.fullName,
  });
  if (profileError) {
    return {
      ok: false,
      error: "Account created, but setting up your profile failed. Try signing in.",
    };
  }

  revalidatePath("/profile");
  // No session means Supabase is set to require email confirmation first.
  return { ok: true, data: { needsConfirmation: !data.session } };
}

export async function signIn(input: SignInInput): Promise<ActionResult<null>> {
  const parsed = SignInSchema.safeParse(input);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0].message, code: "VALIDATION" };
  }

  const supabase = await createClient();
  const { error } = await supabase.auth.signInWithPassword(parsed.data);
  if (error) return { ok: false, error: "Wrong email or password." };

  revalidatePath("/profile");
  return { ok: true, data: null };
}

export async function signOut(): Promise<ActionResult<null>> {
  const supabase = await createClient();
  await supabase.auth.signOut();
  revalidatePath("/profile");
  return { ok: true, data: null };
}
