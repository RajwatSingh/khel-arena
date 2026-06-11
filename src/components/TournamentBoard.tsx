"use client";

// ============================================================================
// TournamentBoard — listing + "Host a tournament" form.
// Left: open competitions as editorial rows — purse in display gold, team
// capacity as a diamond gauge, deadlines in mono. Right: the creation form,
// where format / side count / prize split are segmented controls rather
// than dropdowns, and the prize purse previews live as you type.
// ============================================================================

import { useMemo, useState, useTransition } from "react";
import { AnimatePresence, m } from "framer-motion";
import { PitchDivider } from "@/components/PitchLines";
import type { ActionResult, SkillTier, Tournament, TournamentFormat } from "@/lib/types";
import type { CreateTournamentInput } from "@/actions/tournaments";

export interface TournamentBoardProps {
  tournaments: Tournament[];
  onCreate: (input: CreateTournamentInput) => Promise<ActionResult<Tournament>>;
  onRegister: (tournamentId: string) => Promise<ActionResult<null>>;
}

const NPR = new Intl.NumberFormat("en-IN");
const dateFmt = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Kathmandu",
  day: "2-digit",
  month: "short",
});

const FORMAT_LABEL: Record<TournamentFormat, string> = {
  knockout: "Knockout",
  league: "League",
  group_knockout: "Groups + KO",
};
const SKILL_LABEL: Record<SkillTier, string> = {
  casual: "Casual",
  intermediate: "Intermediate",
  competitive: "Competitive",
  semi_pro: "Semi-pro",
};
const SPLIT_PRESETS: { label: string; split: number[] }[] = [
  { label: "Winner takes all", split: [100] },
  { label: "70 / 30", split: [70, 30] },
  { label: "60 / 30 / 10", split: [60, 30, 10] },
];

const ease = [0.22, 1, 0.36, 1] as const;
const rowAnim = {
  hidden: { opacity: 0, y: 14 },
  show: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.55, ease, delay: i * 0.07 },
  }),
};

function Segmented<T extends string | number>({
  options,
  value,
  onChange,
  label,
}: {
  options: { value: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
  label: string;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-wrap border border-hairline-2">
      {options.map((o) => (
        <button
          key={String(o.value)}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          onClick={() => onChange(o.value)}
          className={`flex-1 whitespace-nowrap px-3 py-2.5 font-mono text-[0.62rem] uppercase tracking-editorial transition-colors ${
            value === o.value ? "bg-ink text-canvas" : "text-ink-dim hover:bg-surface-2 hover:text-ink"
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

const inputClass =
  "w-full border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none";
const labelClass = "eyebrow mb-2 block";

export default function TournamentBoard({ tournaments, onCreate, onRegister }: TournamentBoardProps) {
  // ── Form state ──────────────────────────────────────────────────────────
  const today = new Date().toISOString().slice(0, 10);
  const [name, setName] = useState("");
  const [format, setFormat] = useState<TournamentFormat>("knockout");
  const [sideCount, setSideCount] = useState(5);
  const [maxTeams, setMaxTeams] = useState(8);
  const [entryFee, setEntryFee] = useState(3000);
  const [prizePool, setPrizePool] = useState(50000);
  const [split, setSplit] = useState<number[]>([60, 30, 10]);
  const [skill, setSkill] = useState<SkillTier>("casual");
  const [startsOn, setStartsOn] = useState("");
  const [registerBy, setRegisterBy] = useState("");
  const [description, setDescription] = useState("");

  const [formError, setFormError] = useState<string | null>(null);
  const [created, setCreated] = useState<Tournament | null>(null);
  const [registeredIds, setRegisteredIds] = useState<Set<string>>(new Set());
  const [registerError, setRegisterError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  const gate = useMemo(() => entryFee * maxTeams, [entryFee, maxTeams]);
  const placeLabels = ["Champion", "Runner-up", "Third place", "Fourth place"];

  const handleCreate = () => {
    setFormError(null);
    setCreated(null);
    startTransition(async () => {
      const res = await onCreate({
        name,
        format,
        sideCount,
        squadCap: sideCount * 2,
        maxTeams,
        entryFeeNpr: entryFee,
        prizePoolNpr: prizePool,
        prizeSplit: split,
        skill,
        startsOn,
        registerBy,
        description: description || undefined,
      });
      if (res.ok) {
        setCreated(res.data);
        setName("");
        setDescription("");
      } else {
        setFormError(res.error);
      }
    });
  };

  const handleRegister = (id: string) => {
    setRegisterError(null);
    startTransition(async () => {
      const res = await onRegister(id);
      if (res.ok) setRegisteredIds((prev) => new Set(prev).add(id));
      else setRegisterError(res.error);
    });
  };

  return (
    <section className="relative bg-canvas py-28">
      <div className="mx-auto max-w-6xl px-6">
        <div className="mb-16 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="eyebrow mb-4">Tournaments · प्रतियोगिता</p>
            <h2 className="font-display text-5xl tracking-tight text-ink">Silverware season</h2>
          </div>
          <p className="max-w-xs text-sm leading-relaxed text-ink-dim">
            Enter a cup, or run your own — set the format, the purse, and the
            deadline, and let the valley&rsquo;s teams come to you.
          </p>
        </div>

        <div className="grid gap-16 lg:grid-cols-5">
          {/* ───────────────── Competitions ───────────────── */}
          <div className="lg:col-span-3">
            <div className="mb-6 flex items-baseline justify-between border-b border-hairline-2 pb-4">
              <h3 className="font-mono text-[0.65rem] uppercase tracking-editorial text-ink-dim">
                Open competitions
              </h3>
              <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                {tournaments.length} listed
              </span>
            </div>

            {tournaments.length === 0 ? (
              <div className="border border-dashed border-hairline-2 p-12 text-center">
                <p className="font-display text-xl text-ink-dim">No competitions yet.</p>
                <p className="mt-2 text-sm text-ink-faint">
                  Be the first — the form on the right takes two minutes.
                </p>
              </div>
            ) : (
              <ul>
                {tournaments.map((t, i) => {
                  const spotsLeft = t.max_teams - t.team_count;
                  const closed = t.status !== "open" || spotsLeft <= 0;
                  const done = registeredIds.has(t.id);
                  return (
                    <m.li
                      key={t.id}
                      custom={i}
                      variants={rowAnim}
                      initial="hidden"
                      whileInView="show"
                      viewport={{ once: true, margin: "-60px" }}
                      className="border-b border-hairline py-7"
                    >
                      <div className="flex flex-wrap items-start justify-between gap-5">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-3">
                            <h4 className="font-display text-2xl leading-tight tracking-tight text-ink">
                              {t.name}
                            </h4>
                            <span className="border border-hairline-2 px-2 py-0.5 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-dim">
                              {FORMAT_LABEL[t.format]}
                            </span>
                          </div>
                          <p className="mt-2 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint">
                            {t.side_count}-a-side
                            <span className="mx-3 text-hairline-2">|</span>
                            {t.arena_name ? `${t.arena_name} · ${t.arena_area}` : "Venue TBD"}
                            <span className="mx-3 text-hairline-2">|</span>
                            {SKILL_LABEL[t.skill]}
                          </p>
                          {t.description && (
                            <p className="mt-3 max-w-md text-sm leading-relaxed text-ink-dim">
                              {t.description}
                            </p>
                          )}
                          <p className="mt-3 font-mono text-[0.65rem] uppercase tracking-editorial text-ink-faint">
                            Register by{" "}
                            <span className="text-ink">{dateFmt.format(new Date(t.register_by))}</span>
                            <span className="mx-3 text-hairline-2">|</span>
                            Kicks off{" "}
                            <span className="text-ink">{dateFmt.format(new Date(t.starts_on))}</span>
                            <span className="mx-3 text-hairline-2">|</span>
                            Entry{" "}
                            <span className="text-ink">
                              {t.entry_fee_npr === 0 ? "Free" : `रू ${NPR.format(t.entry_fee_npr)}`}
                            </span>
                          </p>
                        </div>

                        <div className="flex flex-col items-end gap-3">
                          <p className="text-right">
                            <span className="eyebrow block">Prize purse</span>
                            <span className="font-display text-3xl tabular-nums text-gold">
                              रू {NPR.format(t.prize_pool_npr)}
                            </span>
                          </p>
                          {/* Capacity gauge — one diamond per team slot */}
                          <div className="flex max-w-[9rem] flex-wrap justify-end gap-1" aria-hidden>
                            {Array.from({ length: t.max_teams }, (_, n) => (
                              <i
                                key={n}
                                className={`h-1.5 w-1.5 rotate-45 ${
                                  n < t.team_count ? "bg-gold" : "bg-hairline-2"
                                }`}
                              />
                            ))}
                          </div>
                          <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim">
                            {closed
                              ? t.status === "open"
                                ? "Full"
                                : t.status
                              : `${t.team_count}/${t.max_teams} teams`}
                          </span>
                          <button
                            onClick={() => handleRegister(t.id)}
                            disabled={closed || done || isPending}
                            className={`border px-5 py-2.5 font-mono text-[0.62rem] uppercase tracking-editorial transition-colors duration-300 ${
                              done
                                ? "border-sage/60 text-sage"
                                : closed
                                  ? "border-hairline text-ink-faint"
                                  : "border-hairline-2 text-ink-dim hover:border-gold hover:text-gold"
                            } disabled:cursor-default`}
                          >
                            {done ? "Registered" : closed ? "Closed" : "Register team"}
                          </button>
                        </div>
                      </div>
                    </m.li>
                  );
                })}
              </ul>
            )}
            <AnimatePresence>
              {registerError && (
                <m.p
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  role="alert"
                  className="mt-4 font-mono text-xs text-ember"
                >
                  {registerError}
                </m.p>
              )}
            </AnimatePresence>
          </div>

          {/* ───────────────── Host a tournament ───────────────── */}
          <div className="lg:col-span-2">
            <div className="border border-hairline-2 bg-surface p-8">
              <p className="eyebrow mb-2">Host · आयोजना</p>
              <h3 className="font-display text-3xl tracking-tight text-ink">Run your own cup</h3>

              <PitchDivider className="my-7" />

              <div className="space-y-6">
                <div>
                  <label htmlFor="t-name" className={labelClass}>Tournament name</label>
                  <input
                    id="t-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Patan Winter Classic"
                    className={inputClass}
                  />
                </div>

                <div>
                  <span className={labelClass}>Format</span>
                  <Segmented
                    label="Format"
                    value={format}
                    onChange={setFormat}
                    options={[
                      { value: "knockout", label: "Knockout" },
                      { value: "league", label: "League" },
                      { value: "group_knockout", label: "Groups + KO" },
                    ]}
                  />
                </div>

                <div className="grid grid-cols-2 gap-6">
                  <div>
                    <span className={labelClass}>Side count</span>
                    <Segmented
                      label="Side count"
                      value={sideCount}
                      onChange={setSideCount}
                      options={[4, 5, 6, 7].map((n) => ({ value: n, label: `${n}v${n}` }))}
                    />
                  </div>
                  <div>
                    <label htmlFor="t-teams" className={labelClass}>Max teams</label>
                    <input
                      id="t-teams"
                      type="number"
                      min={4}
                      max={32}
                      value={maxTeams}
                      onChange={(e) => setMaxTeams(Number(e.target.value))}
                      className={inputClass}
                    />
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-6">
                  <div>
                    <label htmlFor="t-fee" className={labelClass}>Entry fee (रू)</label>
                    <input
                      id="t-fee"
                      type="number"
                      min={0}
                      step={500}
                      value={entryFee}
                      onChange={(e) => setEntryFee(Number(e.target.value))}
                      className={inputClass}
                    />
                  </div>
                  <div>
                    <label htmlFor="t-prize" className={labelClass}>Prize pool (रू)</label>
                    <input
                      id="t-prize"
                      type="number"
                      min={0}
                      step={5000}
                      value={prizePool}
                      onChange={(e) => setPrizePool(Number(e.target.value))}
                      className={inputClass}
                    />
                  </div>
                </div>

                <div>
                  <span className={labelClass}>Prize split</span>
                  <Segmented
                    label="Prize split"
                    value={split.join("/")}
                    onChange={(v) => setSplit(v.split("/").map(Number))}
                    options={SPLIT_PRESETS.map((p) => ({
                      value: p.split.join("/"),
                      label: p.label,
                    }))}
                  />
                  {/* Live purse preview */}
                  <ul className="mt-3 space-y-1">
                    {split.map((pct, i) => (
                      <li
                        key={i}
                        className="flex justify-between font-mono text-xs tabular-nums text-ink-dim"
                      >
                        <span>{placeLabels[i]}</span>
                        <span className="text-ink">
                          रू {NPR.format(Math.round((prizePool * pct) / 100))}
                          <span className="ml-2 text-ink-faint">{pct}%</span>
                        </span>
                      </li>
                    ))}
                    <li className="flex justify-between border-t border-hairline pt-1 font-mono text-xs tabular-nums text-ink-faint">
                      <span>Gate at capacity</span>
                      <span>रू {NPR.format(gate)}</span>
                    </li>
                  </ul>
                </div>

                <div className="grid grid-cols-2 gap-6">
                  <div>
                    <label htmlFor="t-deadline" className={labelClass}>Register by</label>
                    <input
                      id="t-deadline"
                      type="date"
                      min={today}
                      value={registerBy}
                      onChange={(e) => setRegisterBy(e.target.value)}
                      className={inputClass}
                    />
                  </div>
                  <div>
                    <label htmlFor="t-kickoff" className={labelClass}>Kick-off</label>
                    <input
                      id="t-kickoff"
                      type="date"
                      min={today}
                      value={startsOn}
                      onChange={(e) => setStartsOn(e.target.value)}
                      className={inputClass}
                    />
                  </div>
                </div>

                <div>
                  <span className={labelClass}>Skill level</span>
                  <Segmented
                    label="Skill level"
                    value={skill}
                    onChange={setSkill}
                    options={(Object.keys(SKILL_LABEL) as SkillTier[]).map((k) => ({
                      value: k,
                      label: SKILL_LABEL[k],
                    }))}
                  />
                </div>

                <div>
                  <label htmlFor="t-desc" className={labelClass}>
                    Description <span className="normal-case">(optional)</span>
                  </label>
                  <textarea
                    id="t-desc"
                    rows={3}
                    maxLength={500}
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="What makes this one worth winning?"
                    className={`${inputClass} resize-none`}
                  />
                </div>

                <button
                  onClick={handleCreate}
                  disabled={isPending}
                  className="w-full border border-gold/60 px-6 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink disabled:opacity-50"
                >
                  {isPending ? "Publishing…" : "Publish tournament"}
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
                      {created.name} is live — teams can register now.
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
