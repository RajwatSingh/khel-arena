"use client";

// CreateTeamForm — self-contained "Build a squad" card. Owns its own name/tag
// state; on success it bubbles the new team up so the hub can track its code.
import { useState, useTransition } from "react";
import { AnimatePresence, m } from "framer-motion";
import { PitchDivider } from "@/components/PitchLines";
import { inputClass, labelClass } from "@/lib/ui";
import type { ActionResult, Team } from "@/lib/types";
import type { CreateTeamInput } from "@/actions/teams";

export default function CreateTeamForm({
  onCreateTeam,
  onCreated,
}: {
  onCreateTeam: (input: CreateTeamInput) => Promise<ActionResult<Team>>;
  onCreated: (team: Team) => void;
}) {
  const [name, setName] = useState("");
  const [tag, setTag] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [created, setCreated] = useState<Team | null>(null);
  const [isPending, startTransition] = useTransition();

  const handleCreate = () => {
    setFormError(null);
    setCreated(null);
    startTransition(async () => {
      const res = await onCreateTeam({ name, tag: tag.toUpperCase(), homeArena: null });
      if (res.ok) {
        setCreated(res.data);
        setName("");
        setTag("");
        onCreated(res.data);
      } else {
        setFormError(res.error);
      }
    });
  };

  return (
    <div className="border border-hairline-2 bg-surface p-8">
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
    </div>
  );
}
