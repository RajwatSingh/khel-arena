"use client";

// ProfileClient — /profile composition. In demo mode every action resolves
// locally so the studio is fully explorable; live mode calls the real
// profile server actions.
import ProfileStudio from "@/components/ProfileStudio";
import {
  addHighlight,
  removeHighlight,
  updateProfile,
  uploadAvatar,
  type UpdateProfileInput,
} from "@/actions/profile";
import type { ActionResult, Profile, ProfileHighlight } from "@/lib/types";

interface ProfileClientProps {
  demoMode: boolean;
  profile: Profile;
  highlights: ProfileHighlight[];
}

export default function ProfileClient({ demoMode, profile, highlights }: ProfileClientProps) {
  const onSave = async (input: UpdateProfileInput): Promise<ActionResult<null>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 500));
      return { ok: true, data: null };
    }
    return updateProfile(input);
  };

  const onUploadAvatar = async (fd: FormData): Promise<ActionResult<{ url: string }>> => {
    if (demoMode) return { ok: true, data: { url: "" } };
    return uploadAvatar(fd);
  };

  const onAddHighlight = async (input: {
    title: string;
    url: string;
  }): Promise<ActionResult<ProfileHighlight>> => {
    if (demoMode) {
      await new Promise((r) => setTimeout(r, 300));
      return {
        ok: true,
        data: {
          id: crypto.randomUUID(),
          user_id: profile.id,
          title: input.title,
          url: input.url,
          created_at: new Date().toISOString(),
        },
      };
    }
    return addHighlight(input);
  };

  const onRemoveHighlight = async (id: string): Promise<ActionResult<null>> => {
    if (demoMode) return { ok: true, data: null };
    return removeHighlight(id);
  };

  return (
    <main>
      <ProfileStudio
        demoMode={demoMode}
        profile={profile}
        highlights={highlights}
        onSave={onSave}
        onUploadAvatar={onUploadAvatar}
        onAddHighlight={onAddHighlight}
        onRemoveHighlight={onRemoveHighlight}
      />
    </main>
  );
}
