"use client";

// ============================================================================
// ProfileStudio — the player's editorial card, and the studio to shape it.
// Left: a live preview that updates as you edit — the card other players see.
// Right: the editor — avatar, position (via the court picker), jersey, foot,
// bio, skill, and highlight reels. Everything saves through server actions;
// in demo mode it stays local so the whole studio is explorable offline.
// ============================================================================

import { useRef, useState, useTransition } from "react";
import { AnimatePresence, m } from "framer-motion";
import FutsalPitchPicker from "@/components/FutsalPitchPicker";
import AvatarImage from "@/components/AvatarImage";
import { PitchDivider } from "@/components/PitchLines";
import type {
  ActionResult,
  FutsalPosition,
  Profile,
  ProfileHighlight,
  SkillTier,
} from "@/lib/types";
import type { UpdateProfileInput } from "@/actions/profile";

export interface ProfileStudioProps {
  demoMode: boolean;
  profile: Profile;
  highlights: ProfileHighlight[];
  onSave: (input: UpdateProfileInput) => Promise<ActionResult<null>>;
  onUploadAvatar: (formData: FormData) => Promise<ActionResult<{ url: string }>>;
  onAddHighlight: (input: { title: string; url: string }) => Promise<ActionResult<ProfileHighlight>>;
  onRemoveHighlight: (id: string) => Promise<ActionResult<null>>;
}

const SKILL_LABEL: Record<SkillTier, string> = {
  casual: "Casual",
  intermediate: "Intermediate",
  competitive: "Competitive",
  semi_pro: "Semi-pro",
};
const FOOT_LABEL = { left: "Left", right: "Right", both: "Both" } as const;

const inputClass =
  "w-full border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none";
const labelClass = "eyebrow mb-2 block";

/** Detects the host of a highlight URL for a small provider tag. */
function hostLabel(url: string): string {
  try {
    const h = new URL(url).hostname.replace(/^www\./, "");
    if (h.includes("youtu")) return "YouTube";
    if (h.includes("tiktok")) return "TikTok";
    if (h.includes("instagram")) return "Instagram";
    if (h.includes("drive.google")) return "Drive";
    return h.split(".")[0];
  } catch {
    return "Link";
  }
}

function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
  allowNull,
}: {
  options: { value: T; label: string }[];
  value: T | null;
  onChange: (v: T | null) => void;
  label: string;
  allowNull?: boolean;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-wrap border border-hairline-2">
      {options.map((o) => {
        const selected = value === o.value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(allowNull && selected ? null : o.value)}
            className={`flex-1 whitespace-nowrap px-3 py-2.5 font-mono text-[0.62rem] uppercase tracking-editorial transition-colors ${
              selected ? "bg-ink text-canvas" : "text-ink-dim hover:bg-surface-2 hover:text-ink"
            }`}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

export default function ProfileStudio({
  demoMode,
  profile,
  highlights: initialHighlights,
  onSave,
  onUploadAvatar,
  onAddHighlight,
  onRemoveHighlight,
}: ProfileStudioProps) {
  const [fullName, setFullName] = useState(profile.full_name);
  const [position, setPosition] = useState<FutsalPosition | null>(profile.position);
  const [jersey, setJersey] = useState<number | null>(profile.jersey_number);
  const [foot, setFoot] = useState<"left" | "right" | "both" | null>(profile.preferred_foot);
  const [bio, setBio] = useState(profile.bio ?? "");
  const [skill, setSkill] = useState<SkillTier>(profile.skill);
  const [avatarUrl, setAvatarUrl] = useState<string | null>(profile.avatar_url);

  const [highlights, setHighlights] = useState<ProfileHighlight[]>(initialHighlights);
  const [clipTitle, setClipTitle] = useState("");
  const [clipUrl, setClipUrl] = useState("");

  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();
  const fileRef = useRef<HTMLInputElement>(null);

  const winRate = profile.matches_played
    ? Math.round((profile.matches_won / profile.matches_played) * 100)
    : 0;

  const handleSave = () => {
    setError(null);
    setSaved(false);
    startTransition(async () => {
      const res = await onSave({
        fullName,
        position,
        jerseyNumber: jersey,
        preferredFoot: foot,
        bio: bio || null,
        skill,
      });
      if (res.ok) setSaved(true);
      else setError(res.error);
    });
  };

  const handleAvatar = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setError(null);

    if (demoMode) {
      setAvatarUrl(URL.createObjectURL(file)); // local preview only
      return;
    }
    const fd = new FormData();
    fd.append("avatar", file);
    startTransition(async () => {
      const res = await onUploadAvatar(fd);
      if (res.ok) setAvatarUrl(res.data.url);
      else setError(res.error);
    });
  };

  const handleAddClip = () => {
    setError(null);
    startTransition(async () => {
      const res = await onAddHighlight({ title: clipTitle, url: clipUrl });
      if (res.ok) {
        setHighlights((prev) => [res.data, ...prev]);
        setClipTitle("");
        setClipUrl("");
      } else {
        setError(res.error);
      }
    });
  };

  const handleRemoveClip = (id: string) => {
    setHighlights((prev) => prev.filter((h) => h.id !== id));
    startTransition(async () => {
      await onRemoveHighlight(id);
    });
  };

  const initials = fullName
    .split(" ")
    .map((p) => p[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();

  return (
    <section className="relative bg-canvas py-28">
      <div className="mx-auto max-w-6xl px-6">
        <div className="mb-16">
          <p className="eyebrow mb-4">Profile · खेलाडी</p>
          <h2 className="font-display text-5xl tracking-tight text-ink">Your player card</h2>
        </div>

        <div className="grid gap-16 lg:grid-cols-5">
          {/* ───────────────── Live editorial card ───────────────── */}
          <div className="lg:col-span-2">
            <div className="sticky top-24">
              <p className="eyebrow mb-4">As others see you</p>
              <div className="relative overflow-hidden border border-hairline-2 bg-surface">
                {/* jersey number watermark */}
                {jersey !== null && (
                  <span
                    aria-hidden
                    className="pointer-events-none absolute -right-4 -top-10 select-none font-display text-[12rem] leading-none text-ink"
                    style={{ opacity: 0.05 }}
                  >
                    {jersey}
                  </span>
                )}

                <div className="relative p-8">
                  <div className="flex items-center gap-5">
                    <div className="relative h-20 w-20 shrink-0 overflow-hidden rounded-full border border-hairline-2 bg-surface-2">
                      {avatarUrl ? (
                        <AvatarImage src={avatarUrl} size={80} className="h-full w-full object-cover" />
                      ) : (
                        <span className="flex h-full w-full items-center justify-center font-display text-2xl text-ink-faint">
                          {initials || "—"}
                        </span>
                      )}
                    </div>
                    <div className="min-w-0">
                      <h3 className="truncate font-display text-2xl tracking-tight text-ink">
                        {fullName || "Your name"}
                      </h3>
                      <p className="mt-1 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                        @{profile.username}
                      </p>
                    </div>
                  </div>

                  <PitchDivider className="my-6" />

                  <dl className="grid grid-cols-3 gap-4 text-center">
                    <div>
                      <dt className="eyebrow mb-1">Position</dt>
                      <dd className="font-display text-lg text-gold">{position ?? "—"}</dd>
                    </div>
                    <div>
                      <dt className="eyebrow mb-1">Foot</dt>
                      <dd className="font-display text-lg text-ink">
                        {foot ? FOOT_LABEL[foot] : "—"}
                      </dd>
                    </div>
                    <div>
                      <dt className="eyebrow mb-1">Tier</dt>
                      <dd className="font-display text-lg text-ink">{SKILL_LABEL[skill]}</dd>
                    </div>
                  </dl>

                  {bio && (
                    <p className="mt-6 border-l border-gold/40 pl-4 text-sm italic leading-relaxed text-ink-dim">
                      {bio}
                    </p>
                  )}

                  <div className="mt-6 grid grid-cols-3 gap-px bg-hairline text-center">
                    {[
                      { v: profile.matches_played, l: "Played" },
                      { v: `${winRate}%`, l: "Win rate" },
                      { v: profile.community_score, l: "Rep" },
                    ].map((s) => (
                      <div key={s.l} className="bg-surface py-4">
                        <p className="font-display text-2xl tabular-nums text-ink">{s.v}</p>
                        <p className="eyebrow mt-1">{s.l}</p>
                      </div>
                    ))}
                  </div>

                  {highlights.length > 0 && (
                    <div className="mt-6">
                      <p className="eyebrow mb-3">Highlight reels</p>
                      <ul className="space-y-2">
                        {highlights.slice(0, 3).map((h) => (
                          <li key={h.id}>
                            <a
                              href={h.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="group flex items-center justify-between border-b border-hairline py-2 transition-colors hover:border-gold/40"
                            >
                              <span className="truncate text-sm text-ink-dim transition-colors group-hover:text-ink">
                                {h.title}
                              </span>
                              <span className="ml-3 shrink-0 font-mono text-[0.55rem] uppercase tracking-editorial text-gold">
                                {hostLabel(h.url)} ↗
                              </span>
                            </a>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* ───────────────── The studio ───────────────── */}
          <div className="space-y-10 lg:col-span-3">
            {/* Identity */}
            <div className="border border-hairline-2 bg-surface p-8">
              <h3 className="font-display text-2xl tracking-tight text-ink">Identity</h3>
              <PitchDivider className="my-6" />

              <div className="flex items-center gap-5">
                <div className="relative h-16 w-16 shrink-0 overflow-hidden rounded-full border border-hairline-2 bg-surface-2">
                  {avatarUrl ? (
                    <AvatarImage src={avatarUrl} size={64} className="h-full w-full object-cover" />
                  ) : (
                    <span className="flex h-full w-full items-center justify-center font-display text-xl text-ink-faint">
                      {initials || "—"}
                    </span>
                  )}
                </div>
                <div>
                  <input
                    ref={fileRef}
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    onChange={handleAvatar}
                    className="hidden"
                  />
                  <button
                    type="button"
                    onClick={() => fileRef.current?.click()}
                    className="border border-hairline-2 px-4 py-2 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-gold hover:text-gold"
                  >
                    {avatarUrl ? "Change photo" : "Upload photo"}
                  </button>
                  <p className="mt-2 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
                    JPEG / PNG / WebP · under 4 MB
                  </p>
                </div>
              </div>

              <div className="mt-6">
                <label htmlFor="p-name" className={labelClass}>Full name</label>
                <input
                  id="p-name"
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div className="mt-6">
                <label htmlFor="p-bio" className={labelClass}>
                  Bio <span className="normal-case text-ink-faint">({bio.length}/280)</span>
                </label>
                <textarea
                  id="p-bio"
                  rows={3}
                  maxLength={280}
                  value={bio}
                  onChange={(e) => setBio(e.target.value)}
                  placeholder="Your game in two lines."
                  className={`${inputClass} resize-none`}
                />
              </div>
            </div>

            {/* On the floor */}
            <div className="border border-hairline-2 bg-surface p-8">
              <h3 className="font-display text-2xl tracking-tight text-ink">On the floor</h3>
              <PitchDivider className="my-6" />

              <FutsalPitchPicker value={position} onChange={setPosition} />

              <div className="mt-6 grid gap-6 sm:grid-cols-2">
                <div>
                  <label htmlFor="p-jersey" className={labelClass}>Jersey number</label>
                  <input
                    id="p-jersey"
                    type="number"
                    min={0}
                    max={99}
                    value={jersey ?? ""}
                    onChange={(e) =>
                      setJersey(e.target.value === "" ? null : Number(e.target.value))
                    }
                    placeholder="—"
                    className={inputClass}
                  />
                </div>
                <div>
                  <span className={labelClass}>Preferred foot</span>
                  <Segmented
                    label="Preferred foot"
                    value={foot}
                    onChange={setFoot}
                    allowNull
                    options={[
                      { value: "left", label: "Left" },
                      { value: "right", label: "Right" },
                      { value: "both", label: "Both" },
                    ]}
                  />
                </div>
              </div>

              <div className="mt-6">
                <span className={labelClass}>Skill tier</span>
                <Segmented
                  label="Skill tier"
                  value={skill}
                  onChange={(v) => v && setSkill(v)}
                  options={(Object.keys(SKILL_LABEL) as SkillTier[]).map((k) => ({
                    value: k,
                    label: SKILL_LABEL[k],
                  }))}
                />
              </div>
            </div>

            {/* Highlight reels */}
            <div className="border border-hairline-2 bg-surface p-8">
              <h3 className="font-display text-2xl tracking-tight text-ink">Highlight reels</h3>
              <p className="mt-2 text-sm text-ink-dim">
                Paste links to your best clips — YouTube, TikTok, Instagram, Drive.
              </p>
              <PitchDivider className="my-6" />

              {highlights.length > 0 && (
                <ul className="mb-6 space-y-3">
                  <AnimatePresence initial={false}>
                    {highlights.map((h) => (
                      <m.li
                        key={h.id}
                        layout
                        initial={{ opacity: 0, y: 8 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, height: 0 }}
                        className="flex items-center justify-between border border-hairline bg-canvas px-4 py-3"
                      >
                        <div className="min-w-0">
                          <p className="truncate text-sm text-ink">{h.title}</p>
                          <p className="truncate font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
                            {hostLabel(h.url)} · {h.url}
                          </p>
                        </div>
                        <button
                          onClick={() => handleRemoveClip(h.id)}
                          aria-label={`Remove ${h.title}`}
                          className="ml-4 shrink-0 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint transition-colors hover:text-ember"
                        >
                          Remove
                        </button>
                      </m.li>
                    ))}
                  </AnimatePresence>
                </ul>
              )}

              <div className="grid gap-4 sm:grid-cols-[1fr_1.4fr_auto] sm:items-end">
                <div>
                  <label htmlFor="c-title" className={labelClass}>Clip title</label>
                  <input
                    id="c-title"
                    value={clipTitle}
                    onChange={(e) => setClipTitle(e.target.value)}
                    placeholder="Hat-trick vs Bulls"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label htmlFor="c-url" className={labelClass}>Link</label>
                  <input
                    id="c-url"
                    value={clipUrl}
                    onChange={(e) => setClipUrl(e.target.value)}
                    placeholder="https://youtube.com/watch?v=…"
                    className={inputClass}
                  />
                </div>
                <button
                  onClick={handleAddClip}
                  disabled={isPending || !clipTitle || !clipUrl}
                  className="border border-hairline-2 px-5 py-3 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-gold hover:text-gold disabled:opacity-40"
                >
                  Add clip
                </button>
              </div>
            </div>

            {/* Save bar */}
            <div className="flex flex-wrap items-center gap-6">
              <button
                onClick={handleSave}
                disabled={isPending}
                className="border border-gold/60 px-8 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink disabled:opacity-50"
              >
                {isPending ? "Saving…" : "Save profile"}
              </button>
              <AnimatePresence>
                {saved && (
                  <m.span
                    initial={{ opacity: 0, x: -6 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0 }}
                    className="font-mono text-xs text-sage"
                    role="status"
                  >
                    Saved{demoMode ? " (demo — local only)" : ""}.
                  </m.span>
                )}
                {error && (
                  <m.span
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    className="font-mono text-xs text-ember"
                    role="alert"
                  >
                    {error}
                  </m.span>
                )}
              </AnimatePresence>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
