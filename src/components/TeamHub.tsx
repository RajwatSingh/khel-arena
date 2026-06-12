"use client";

// ============================================================================
// TeamHub — /teams composition.
// Left: a sticky team crest card (mirroring ProfileStudio's live preview)
// that shows the selected team's roster as a formation sheet — names,
// roles, the join code as a tactical annotation, and a diamond capacity
// gauge from TournamentBoard. Clicking a team selects it.
// Right: "Your squads" listing (editorial rows identical to TournamentBoard)
// plus creation form and join-by-code, both in bordered surface cards with
// PitchDividers. The entire section sits on the grain canvas with a
// Devanagari watermark.
// ============================================================================

import { useEffect, useRef, useState, useTransition } from "react";
import { AnimatePresence, m } from "framer-motion";
import AvatarImage from "@/components/AvatarImage";
import { PitchBackdrop, PitchDivider } from "@/components/PitchLines";
import type { ActionResult, Team, TeamMember } from "@/lib/types";
import type { CreateTeamInput } from "@/actions/teams";

export interface TeamHubProps {
  teams: Team[];
  onCreateTeam: (input: CreateTeamInput) => Promise<ActionResult<Team>>;
  onGetMembers: (teamId: string) => Promise<ActionResult<{ team: Team; members: TeamMember[] }>>;
  onAddMember: (teamId: string, username: string) => Promise<ActionResult<TeamMember>>;
  onRemoveMember: (teamId: string, userId: string) => Promise<ActionResult<null>>;
  onJoinByCode: (code: string) => Promise<ActionResult<Team>>;
  onRegenerateCode: (teamId: string) => Promise<ActionResult<string>>;
}

const inputClass =
  "w-full border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none";
const labelClass = "eyebrow mb-2 block";

const ease = [0.22, 1, 0.36, 1] as const;

export default function TeamHub({
  teams,
  onCreateTeam,
  onGetMembers,
  onAddMember,
  onRemoveMember,
  onJoinByCode,
  onRegenerateCode,
}: TeamHubProps) {
  // ── Create form state ─────────────────────────────────────────────────────
  const [name, setName] = useState("");
  const [tag, setTag] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [created, setCreated] = useState<Team | null>(null);
  const [isPending, startTransition] = useTransition();
  const [showCreate, setShowCreate] = useState(false);

  // ── Join code state ───────────────────────────────────────────────────────
  const [joinCode, setJoinCode] = useState("");
  const [joinError, setJoinError] = useState<string | null>(null);
  const [joinSuccess, setJoinSuccess] = useState<string | null>(null);

  // ── Selected team + roster state ──────────────────────────────────────────
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(
    teams.length > 0 ? teams[0].id : null
  );
  const [roster, setRoster] = useState<TeamMember[]>([]);
  const [rosterLoading, setRosterLoading] = useState(false);
  const [rosterLoaded, setRosterLoaded] = useState<string | null>(null);
  const [addUsername, setAddUsername] = useState("");
  const [addError, setAddError] = useState<string | null>(null);
  const [teamJoinCodes, setTeamJoinCodes] = useState<Record<string, string>>(() => {
    const map: Record<string, string> = {};
    teams.forEach((t) => {
      map[t.id] = t.join_code;
    });
    return map;
  });
  const [copied, setCopied] = useState<string | null>(null);

  const selectedTeam = teams.find((t) => t.id === selectedTeamId) ?? null;

  const handleSelect = async (teamId: string) => {
    setSelectedTeamId(teamId);
    setAddError(null);
    setAddUsername("");
    if (rosterLoaded === teamId) return; // already loaded
    setRoster([]);
    setRosterLoading(true);
    const res = await onGetMembers(teamId);
    setRosterLoading(false);
    if (res.ok) {
      setRoster(res.data.members);
      setRosterLoaded(teamId);
      setTeamJoinCodes((prev) => ({ ...prev, [teamId]: res.data.team.join_code }));
    }
  };

  const handleCreate = () => {
    setFormError(null);
    setCreated(null);
    startTransition(async () => {
      const res = await onCreateTeam({ name, tag: tag.toUpperCase(), homeArena: null });
      if (res.ok) {
        setCreated(res.data);
        setName("");
        setTag("");
        setTeamJoinCodes((prev) => ({ ...prev, [res.data.id]: res.data.join_code }));
      } else {
        setFormError(res.error);
      }
    });
  };

  const handleJoin = () => {
    setJoinError(null);
    setJoinSuccess(null);
    startTransition(async () => {
      const res = await onJoinByCode(joinCode);
      if (res.ok) {
        setJoinSuccess(`Joined ${res.data.name}!`);
        setJoinCode("");
      } else {
        setJoinError(res.error);
      }
    });
  };

  const handleAddMember = () => {
    if (!selectedTeamId) return;
    setAddError(null);
    startTransition(async () => {
      const res = await onAddMember(selectedTeamId!, addUsername);
      if (res.ok) {
        setRoster((prev) => [...prev, res.data]);
        setAddUsername("");
      } else {
        setAddError(res.error);
      }
    });
  };

  const handleRemoveMember = (userId: string) => {
    if (!selectedTeamId) return;
    startTransition(async () => {
      const res = await onRemoveMember(selectedTeamId!, userId);
      if (res.ok) {
        setRoster((prev) => prev.filter((m) => m.user_id !== userId));
      }
    });
  };

  const handleRegenCode = () => {
    if (!selectedTeamId) return;
    startTransition(async () => {
      const res = await onRegenerateCode(selectedTeamId!);
      if (res.ok) {
        setTeamJoinCodes((prev) => ({ ...prev, [selectedTeamId!]: res.data }));
      }
    });
  };

  const copyCode = (code: string) => {
    navigator.clipboard.writeText(code);
    setCopied(code);
    setTimeout(() => setCopied(null), 1500);
  };

  // Close the build-a-squad modal on Escape.
  useEffect(() => {
    if (!showCreate) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setShowCreate(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [showCreate]);

  // Team picker dropdown: close on outside click or Escape.
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!menuOpen) return;
    const onDown = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [menuOpen]);

  // If a team was just created, auto-select it
  if (created && !teams.find((t) => t.id === created.id)) {
    // Will be in the list on next render via parent state
  }

  const selectedCode = selectedTeamId ? teamJoinCodes[selectedTeamId] ?? selectedTeam?.join_code : null;

  return (
    <section className="grain relative min-h-screen overflow-hidden bg-canvas py-28">
      {/* Court markings backdrop */}
      <PitchBackdrop />

      {/* Devanagari watermark */}
      <m.span
        aria-hidden
        initial={{ opacity: 0 }}
        animate={{ opacity: 0.04 }}
        transition={{ duration: 2, delay: 0.6 }}
        className="absolute -right-8 top-1/2 -translate-y-1/2 select-none font-display text-[22rem] leading-none text-ink"
      >
        टोली
      </m.span>

      <div className="relative mx-auto max-w-6xl px-6">
        {/* Page header */}
        <div className="mb-16 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="eyebrow mb-4">Squads &middot; टोली</p>
            <h2 className="font-display text-5xl tracking-tight text-ink">
              Your <em className="not-italic text-gold">squads</em>
            </h2>
          </div>
          <div className="flex flex-col gap-4 sm:items-end">
            <p className="max-w-xs text-sm leading-relaxed text-ink-dim">
              Build a squad, invite players by username or share a join code,
              then register for tournaments together.
            </p>
            <button
              type="button"
              onClick={() => setShowCreate(true)}
              className="shrink-0 border border-gold/60 px-6 py-3 font-mono text-[0.65rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink"
            >
              Build a squad
            </button>
          </div>
        </div>

        <div className="grid gap-16 lg:grid-cols-5">
          {/* ────────────── LEFT: Team roster card (sticky) ────────────── */}
          <div className="lg:col-span-2">
            <div className="sticky top-24">
              <p className="eyebrow mb-4">
                {selectedTeam ? "Formation sheet" : "Select a squad"}
              </p>

              <div className="relative overflow-hidden border border-hairline-2 bg-surface">
                {/* Tag watermark, like jersey number on ProfileStudio */}
                {selectedTeam && (
                  <span
                    aria-hidden
                    className="pointer-events-none absolute -right-3 -top-6 select-none font-display text-[10rem] leading-none text-ink"
                    style={{ opacity: 0.04 }}
                  >
                    {selectedTeam.tag}
                  </span>
                )}

                <div className="relative p-8">
                  {!selectedTeam ? (
                    <div className="py-12 text-center">
                      <p className="font-display text-xl text-ink-dim">No squad selected</p>
                      <p className="mt-2 text-sm text-ink-faint">
                        Pick one from the list, or create your first.
                      </p>
                    </div>
                  ) : (
                    <>
                      {/* Team header */}
                      <div className="flex items-start justify-between">
                        <div>
                          <h3 className="font-display text-3xl tracking-tight text-ink">
                            {selectedTeam.name}
                          </h3>
                          <p className="mt-1 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                            Est. {new Date(selectedTeam.created_at).getFullYear()}
                          </p>
                        </div>
                        <span className="border border-gold/40 px-3 py-1.5 font-mono text-[0.7rem] font-semibold uppercase tracking-editorial text-gold">
                          {selectedTeam.tag}
                        </span>
                      </div>

                      <PitchDivider className="my-6" />

                      {/* Stats row */}
                      <div className="grid grid-cols-3 gap-px bg-hairline text-center">
                        <div className="bg-surface py-4">
                          <p className="font-display text-2xl tabular-nums text-ink">
                            {roster.length || selectedTeam.member_count}
                          </p>
                          <p className="eyebrow mt-1">Players</p>
                        </div>
                        <div className="bg-surface py-4">
                          <p className="font-display text-2xl tabular-nums text-gold">
                            {roster.filter((m) => m.role === "captain").length || 1}
                          </p>
                          <p className="eyebrow mt-1">Captain</p>
                        </div>
                        <div className="bg-surface py-4">
                          <p className="font-display text-2xl tabular-nums text-ink">
                            {selectedTeam.member_count}
                          </p>
                          <p className="eyebrow mt-1">Roster</p>
                        </div>
                      </div>

                      {/* Roster list */}
                      <div className="mt-6">
                        <p className="eyebrow mb-3">Squad roster</p>
                        {rosterLoading ? (
                          <div className="space-y-2">
                            {[1, 2, 3].map((n) => (
                              <div
                                key={n}
                                className="h-14 animate-pulse border border-hairline bg-canvas"
                              />
                            ))}
                          </div>
                        ) : (
                          <AnimatePresence initial={false}>
                            <ul className="space-y-2">
                              {roster.map((member, i) => (
                                <m.li
                                  key={member.user_id}
                                  layout
                                  initial={{ opacity: 0, y: 8 }}
                                  animate={{ opacity: 1, y: 0 }}
                                  exit={{ opacity: 0, height: 0 }}
                                  transition={{ delay: i * 0.04 }}
                                  className="flex items-center justify-between border border-hairline bg-canvas px-4 py-3"
                                >
                                  <div className="flex items-center gap-3">
                                    {/* Avatar circle */}
                                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-hairline-2 bg-surface-2">
                                      {member.avatar_url ? (
                                        <AvatarImage
                                          src={member.avatar_url}
                                          size={32}
                                          className="h-full w-full rounded-full object-cover"
                                        />
                                      ) : (
                                        <span className="font-display text-xs text-ink-faint">
                                          {member.full_name
                                            .split(" ")
                                            .map((p) => p[0])
                                            .slice(0, 2)
                                            .join("")
                                            .toUpperCase()}
                                        </span>
                                      )}
                                    </div>
                                    <div className="min-w-0">
                                      <p className="truncate text-sm font-medium text-ink">
                                        {member.full_name}
                                      </p>
                                      <p className="font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
                                        @{member.username}
                                      </p>
                                    </div>
                                  </div>
                                  <div className="flex items-center gap-3">
                                    {member.role === "captain" ? (
                                      <span className="border border-gold/40 px-2 py-0.5 font-mono text-[0.55rem] uppercase tracking-editorial text-gold">
                                        Captain
                                      </span>
                                    ) : (
                                      <span className="font-mono text-[0.55rem] uppercase tracking-editorial text-ink-faint">
                                        Player
                                      </span>
                                    )}
                                    {member.role !== "captain" && (
                                      <button
                                        onClick={() => handleRemoveMember(member.user_id)}
                                        disabled={isPending}
                                        className="font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint transition-colors hover:text-ember"
                                      >
                                        &times;
                                      </button>
                                    )}
                                  </div>
                                </m.li>
                              ))}
                            </ul>
                          </AnimatePresence>
                        )}
                      </div>

                      {/* Add player inline */}
                      <div className="mt-4 flex gap-2">
                        <input
                          value={addUsername}
                          onChange={(e) => setAddUsername(e.target.value)}
                          placeholder="Add by username..."
                          className={`${inputClass} flex-1`}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" && addUsername.trim()) handleAddMember();
                          }}
                        />
                        <button
                          onClick={handleAddMember}
                          disabled={!addUsername.trim() || isPending}
                          className="border border-hairline-2 px-4 py-3 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-gold hover:text-gold disabled:opacity-40"
                        >
                          Add
                        </button>
                      </div>
                      <AnimatePresence>
                        {addError && (
                          <m.p
                            initial={{ opacity: 0 }}
                            animate={{ opacity: 1 }}
                            exit={{ opacity: 0 }}
                            className="mt-2 font-mono text-xs text-ember"
                          >
                            {addError}
                          </m.p>
                        )}
                      </AnimatePresence>

                      {/* Join code */}
                      <div className="mt-6">
                        <p className="eyebrow mb-3">Invite code</p>
                        <div className="flex items-center gap-3 border border-dashed border-hairline-2 bg-canvas px-4 py-3">
                          <span className="flex-1 font-mono text-lg tracking-[0.2em] text-ink">
                            {selectedCode}
                          </span>
                          <button
                            onClick={() => selectedCode && copyCode(selectedCode)}
                            className="border border-hairline-2 px-3 py-1.5 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-gold hover:text-gold"
                          >
                            {copied === selectedCode ? "Copied" : "Copy"}
                          </button>
                          <button
                            onClick={handleRegenCode}
                            disabled={isPending}
                            className="font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint transition-colors hover:text-ink"
                          >
                            Reset
                          </button>
                        </div>
                      </div>
                    </>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* ────────────── RIGHT: Squads list + forms ────────────── */}
          <div className="space-y-10 lg:col-span-3">
            {/* ── Squad listing ──────────────────────────────────── */}
            <div>
              <div className="mb-6 flex items-baseline justify-between border-b border-hairline-2 pb-4">
                <h3 className="font-mono text-[0.65rem] uppercase tracking-editorial text-ink-dim">
                  Your squads
                </h3>
                <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                  {teams.length} {teams.length === 1 ? "team" : "teams"}
                </span>
              </div>

              {teams.length === 0 ? (
                <div className="border border-dashed border-hairline-2 p-12 text-center">
                  <p className="font-display text-xl text-ink-dim">No squads yet.</p>
                  <p className="mt-2 text-sm text-ink-faint">
                    Build your first with the button above — it takes thirty seconds.
                  </p>
                </div>
              ) : (
                <div className="relative" ref={menuRef}>
                  {/* Themed team picker */}
                  <button
                    type="button"
                    aria-haspopup="listbox"
                    aria-expanded={menuOpen}
                    onClick={() => setMenuOpen((o) => !o)}
                    className="flex w-full items-center justify-between gap-4 border border-hairline-2 bg-surface px-5 py-4 text-left transition-colors hover:border-gold"
                  >
                    <span className="flex min-w-0 items-center gap-3">
                      <span className="truncate font-display text-lg tracking-tight text-ink">
                        {selectedTeam ? selectedTeam.name : "Select a squad"}
                      </span>
                      {selectedTeam && (
                        <span className="shrink-0 border border-gold/60 px-2 py-0.5 font-mono text-[0.55rem] uppercase tracking-editorial text-gold">
                          {selectedTeam.tag}
                        </span>
                      )}
                    </span>
                    <svg
                      aria-hidden
                      viewBox="0 0 24 24"
                      className={`h-4 w-4 shrink-0 text-ink-dim transition-transform duration-200 ${
                        menuOpen ? "rotate-180" : ""
                      }`}
                      fill="none"
                      stroke="currentColor"
                      strokeWidth={1.8}
                    >
                      <path d="m6 9 6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>

                  <AnimatePresence>
                    {menuOpen && (
                      <m.ul
                        role="listbox"
                        initial={{ opacity: 0, y: -6 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: -6 }}
                        transition={{ duration: 0.18, ease }}
                        className="absolute z-30 mt-2 max-h-80 w-full overflow-y-auto border border-hairline-2 bg-surface shadow-xl"
                      >
                        {teams.map((t) => {
                          const isSelected = selectedTeamId === t.id;
                          return (
                            <li key={t.id} role="option" aria-selected={isSelected}>
                              <button
                                type="button"
                                onClick={() => {
                                  handleSelect(t.id);
                                  setMenuOpen(false);
                                }}
                                className={`flex w-full items-center justify-between gap-3 border-b border-hairline px-5 py-3.5 text-left transition-colors last:border-b-0 ${
                                  isSelected ? "bg-surface-2" : "hover:bg-surface-2"
                                }`}
                              >
                                <span className="flex min-w-0 items-center gap-3">
                                  <span
                                    className={`truncate font-display text-base tracking-tight ${
                                      isSelected ? "text-gold" : "text-ink"
                                    }`}
                                  >
                                    {t.name}
                                  </span>
                                  <span className="shrink-0 border border-hairline-2 px-1.5 py-0.5 font-mono text-[0.52rem] uppercase tracking-editorial text-ink-dim">
                                    {t.tag}
                                  </span>
                                </span>
                                <span className="shrink-0 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
                                  {t.member_count} {t.member_count === 1 ? "player" : "players"}
                                </span>
                              </button>
                            </li>
                          );
                        })}
                      </m.ul>
                    )}
                  </AnimatePresence>
                </div>
              )}
            </div>

            {/* ── Build a squad (modal) ──────────────────────────── */}
            <AnimatePresence>
              {showCreate && (
                <m.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  className="fixed inset-0 z-[60] flex items-start justify-center overflow-y-auto bg-ink/40 px-6 py-10 backdrop-blur-sm"
                  onClick={() => setShowCreate(false)}
                >
                <m.div
                  role="dialog"
                  aria-modal="true"
                  aria-label="Build a squad"
                  initial={{ opacity: 0, y: 24, scale: 0.97 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: 12, scale: 0.98 }}
                  transition={{ duration: 0.35, ease }}
                  className="relative w-full max-w-md border border-hairline-2 bg-surface p-8"
                  onClick={(e) => e.stopPropagation()}
                >
                <button
                  type="button"
                  onClick={() => setShowCreate(false)}
                  aria-label="Close"
                  className="absolute right-5 top-5 font-mono text-sm text-ink-faint transition-colors hover:text-ink"
                >
                  ✕
                </button>
                <p className="eyebrow mb-2">Create &middot; बनाउनुहोस्</p>
                <h3 className="font-display text-3xl tracking-tight text-ink">Build a squad</h3>

                <PitchDivider className="my-7" />

              <div className="space-y-6">
                <div>
                  <label htmlFor="team-name" className={labelClass}>
                    Team name
                  </label>
                  <input
                    id="team-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Kathmandu United"
                    maxLength={40}
                    className={inputClass}
                  />
                </div>

                <div>
                  <label htmlFor="team-tag" className={labelClass}>
                    Tag
                  </label>
                  <input
                    id="team-tag"
                    value={tag}
                    onChange={(e) =>
                      setTag(
                        e.target.value
                          .toUpperCase()
                          .replace(/[^A-Z0-9]/g, "")
                          .slice(0, 5)
                      )
                    }
                    placeholder="KTM"
                    maxLength={5}
                    className={inputClass}
                  />
                  <p className="mt-2 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
                    2-5 uppercase letters or digits &middot; shown on jerseys &amp; brackets
                    {tag.length > 0 && tag.length < 2 && (
                      <span className="ml-2 text-ember">&middot; too short</span>
                    )}
                  </p>
                  {/* Live tag preview */}
                  {tag.length >= 2 && (
                    <m.div
                      initial={{ opacity: 0, scale: 0.95 }}
                      animate={{ opacity: 1, scale: 1 }}
                      className="mt-3 inline-flex border border-gold/40 px-4 py-2 font-mono text-sm font-semibold uppercase tracking-editorial text-gold"
                    >
                      {tag}
                    </m.div>
                  )}
                </div>

                <button
                  onClick={handleCreate}
                  disabled={isPending || !name.trim() || tag.length < 2}
                  className="w-full border border-gold/60 px-6 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink disabled:opacity-50"
                >
                  {isPending ? "Creating..." : "Create team"}
                </button>

                <AnimatePresence>
                  {formError && (
                    <m.p
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      role="alert"
                      className="font-mono text-xs text-ember"
                    >
                      {formError}
                    </m.p>
                  )}
                  {created && (
                    <m.p
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      role="status"
                      className="font-mono text-xs text-sage"
                    >
                      {created.name} created — share code{" "}
                      <span className="tracking-wider text-ink">{created.join_code}</span> with your
                      players.
                    </m.p>
                  )}
                </AnimatePresence>
              </div>
                </m.div>
                </m.div>
              )}
            </AnimatePresence>

            {/* ── Join a team ────────────────────────────────────── */}
            <div className="border border-hairline-2 bg-surface p-8">
              <p className="eyebrow mb-2">Join &middot; सामेल हुनुहोस्</p>
              <h3 className="font-display text-3xl tracking-tight text-ink">Got an invite?</h3>

              <PitchDivider className="my-7" />

              <div className="space-y-6">
                <div>
                  <label htmlFor="join-code" className={labelClass}>
                    Join code
                  </label>
                  <input
                    id="join-code"
                    value={joinCode}
                    onChange={(e) => setJoinCode(e.target.value.toUpperCase())}
                    placeholder="JFC-X7K2"
                    className={`${inputClass} font-mono tracking-[0.15em]`}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && joinCode.trim()) handleJoin();
                    }}
                  />
                  <p className="mt-2 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
                    Ask your captain for the code
                  </p>
                </div>

                <button
                  onClick={handleJoin}
                  disabled={isPending || !joinCode.trim()}
                  className="w-full border border-hairline-2 px-6 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-ink-dim transition-colors duration-300 hover:border-gold hover:text-gold disabled:opacity-50"
                >
                  {isPending ? "Joining..." : "Join team"}
                </button>

                <AnimatePresence>
                  {joinError && (
                    <m.p
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      role="alert"
                      className="font-mono text-xs text-ember"
                    >
                      {joinError}
                    </m.p>
                  )}
                  {joinSuccess && (
                    <m.p
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      role="status"
                      className="font-mono text-xs text-sage"
                    >
                      {joinSuccess}
                    </m.p>
                  )}
                </AnimatePresence>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
