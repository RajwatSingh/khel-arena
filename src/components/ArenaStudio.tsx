"use client";

// ============================================================================
// ArenaStudio — the futsal owner's dashboard on /profile.
// First visit: a create form for the arena profile (name, area, story, hours).
// After that: edit the same details, tune opening hours, and manage courts —
// add new ones and reprice existing ones. Everything saves through the arena
// server actions; RLS keeps owners inside their own arena.
// ============================================================================

import { useState, useTransition } from "react";
import { AnimatePresence, m } from "framer-motion";
import { PitchDivider } from "@/components/PitchLines";
import {
  addCourt,
  createArena,
  updateArena,
  updateCourtPrice,
  type ArenaInput,
} from "@/actions/arena";
import type { Arena, Court } from "@/lib/types";

const inputClass =
  "w-full border border-hairline-2 bg-surface px-4 py-3 text-sm text-ink placeholder:text-ink-faint focus:border-gold focus:outline-none";
const labelClass = "eyebrow mb-2 block";
const buttonClass =
  "border border-gold/60 px-8 py-4 font-mono text-[0.68rem] uppercase tracking-editorial text-gold transition-colors duration-300 hover:bg-gold hover:text-ink disabled:opacity-50";

const toHHMM = (t: string) => t.slice(0, 5);

function Feedback({ error, notice }: { error: string | null; notice: string | null }) {
  return (
    <AnimatePresence>
      {error && (
        <m.p
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0 }}
          className="font-mono text-xs text-ember"
          role="alert"
        >
          {error}
        </m.p>
      )}
      {notice && (
        <m.p
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0 }}
          className="font-mono text-xs text-sage"
          role="status"
        >
          {notice}
        </m.p>
      )}
    </AnimatePresence>
  );
}

function CourtRow({ court }: { court: Court }) {
  const [price, setPrice] = useState(String(court.base_price));
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  const dirty = Number(price) !== court.base_price;

  const save = () =>
    startTransition(async () => {
      setError(null);
      setNotice(null);
      const res = await updateCourtPrice(court.id, Number(price));
      if (res.ok) setNotice("Price updated.");
      else setError(res.error);
    });

  return (
    <div className="border border-hairline-2 px-4 py-4">
      <div className="flex flex-wrap items-center gap-4">
        <div className="min-w-0 flex-1">
          <p className="text-sm text-ink">{court.label}</p>
          <p className="mt-1 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
            {court.side_count}-a-side · {court.sport}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
            Rs.
          </span>
          <input
            type="number"
            min={100}
            step={50}
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            aria-label={`Hourly price for ${court.label}`}
            className="w-28 border border-hairline-2 bg-surface px-3 py-2 text-sm text-ink focus:border-gold focus:outline-none"
          />
          <span className="font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
            /hr
          </span>
          <button
            type="button"
            onClick={save}
            disabled={isPending || !dirty}
            className="border border-hairline-2 px-4 py-2 font-mono text-[0.62rem] uppercase tracking-editorial text-ink-dim transition-colors hover:border-gold hover:text-gold disabled:opacity-40"
          >
            {isPending ? "Saving…" : "Save"}
          </button>
        </div>
      </div>
      <div className="mt-2">
        <Feedback error={error} notice={notice} />
      </div>
    </div>
  );
}

export default function ArenaStudio({
  arena: initialArena,
  courts: initialCourts,
}: {
  arena: Arena | null;
  courts: Court[];
}) {
  const [arena, setArena] = useState<Arena | null>(initialArena);
  const [courts, setCourts] = useState<Court[]>(initialCourts);

  const [name, setName] = useState(initialArena?.name ?? "");
  const [area, setArea] = useState(initialArena?.area ?? "");
  const [city, setCity] = useState(initialArena?.city ?? "Kathmandu");
  const [description, setDescription] = useState(initialArena?.description ?? "");
  const [amenities, setAmenities] = useState(initialArena?.amenities.join(", ") ?? "");
  const [opensAt, setOpensAt] = useState(initialArena ? toHHMM(initialArena.opens_at) : "06:00");
  const [closesAt, setClosesAt] = useState(initialArena ? toHHMM(initialArena.closes_at) : "22:00");

  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  const [courtLabel, setCourtLabel] = useState("");
  const [courtSide, setCourtSide] = useState("5");
  const [courtPrice, setCourtPrice] = useState("1500");
  const [courtError, setCourtError] = useState<string | null>(null);
  const [courtPending, startCourtTransition] = useTransition();

  const arenaInput = (): ArenaInput => ({
    name,
    area,
    city,
    description: description.trim() === "" ? null : description,
    amenities: amenities
      .split(",")
      .map((a) => a.trim())
      .filter(Boolean),
    opensAt,
    closesAt,
  });

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setNotice(null);
    startTransition(async () => {
      if (!arena) {
        const res = await createArena(arenaInput());
        if (res.ok) {
          setArena(res.data);
          setNotice("Your arena is live. Add your courts below.");
        } else setError(res.error);
        return;
      }
      const res = await updateArena(arenaInput());
      if (res.ok) setNotice("Arena saved.");
      else setError(res.error);
    });
  };

  const handleAddCourt = (e: React.FormEvent) => {
    e.preventDefault();
    setCourtError(null);
    startCourtTransition(async () => {
      const res = await addCourt({
        label: courtLabel,
        sideCount: Number(courtSide),
        basePrice: Number(courtPrice),
      });
      if (res.ok) {
        setCourts((prev) => [...prev, res.data]);
        setCourtLabel("");
      } else setCourtError(res.error);
    });
  };

  return (
    <section className="relative bg-canvas py-20">
      <div className="mx-auto max-w-3xl px-6">
        <p className="eyebrow mb-4">Arena studio · मैदान</p>
        <h2 className="font-display text-5xl tracking-tight text-ink">
          {arena ? arena.name : "List your futsal"}
        </h2>
        <p className="mt-4 max-w-lg text-sm leading-relaxed text-ink-dim">
          {arena
            ? "Tune your arena profile, opening hours, and court prices. Changes go live on the booking page immediately."
            : "Set up your arena profile — players will find it on the booking page once it's live."}
        </p>

        {/* Arena profile + hours */}
        <form onSubmit={handleSave} className="mt-12 space-y-6">
          <div>
            <label htmlFor="ar-name" className={labelClass}>Arena name</label>
            <input
              id="ar-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Dhuku Futsal"
              className={inputClass}
            />
          </div>

          <div className="grid gap-6 sm:grid-cols-2">
            <div>
              <label htmlFor="ar-area" className={labelClass}>Area</label>
              <input
                id="ar-area"
                value={area}
                onChange={(e) => setArea(e.target.value)}
                placeholder="Jhamsikhel"
                className={inputClass}
              />
            </div>
            <div>
              <label htmlFor="ar-city" className={labelClass}>City</label>
              <input
                id="ar-city"
                value={city}
                onChange={(e) => setCity(e.target.value)}
                className={inputClass}
              />
            </div>
          </div>

          <div>
            <label htmlFor="ar-desc" className={labelClass}>About the arena</label>
            <textarea
              id="ar-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              maxLength={500}
              placeholder="Rooftop turf with floodlights, five minutes from Jhamsikhel chowk…"
              className={`${inputClass} resize-none`}
            />
          </div>

          <div>
            <label htmlFor="ar-amenities" className={labelClass}>Amenities</label>
            <input
              id="ar-amenities"
              value={amenities}
              onChange={(e) => setAmenities(e.target.value)}
              placeholder="parking, showers, floodlights"
              className={inputClass}
            />
            <p className="mt-2 font-mono text-[0.58rem] uppercase tracking-editorial text-ink-faint">
              Separate with commas
            </p>
          </div>

          <div className="grid gap-6 sm:grid-cols-2">
            <div>
              <label htmlFor="ar-opens" className={labelClass}>Opens at</label>
              <input
                id="ar-opens"
                type="time"
                value={opensAt}
                onChange={(e) => setOpensAt(e.target.value)}
                className={inputClass}
              />
            </div>
            <div>
              <label htmlFor="ar-closes" className={labelClass}>Closes at</label>
              <input
                id="ar-closes"
                type="time"
                value={closesAt}
                onChange={(e) => setClosesAt(e.target.value)}
                className={inputClass}
              />
            </div>
          </div>

          <button type="submit" disabled={isPending} className={`w-full ${buttonClass}`}>
            {isPending
              ? "Saving…"
              : arena
                ? "Save arena"
                : "Create arena profile"}
          </button>

          <Feedback error={error} notice={notice} />
        </form>

        {/* Courts & pricing — only once the arena exists */}
        {arena && (
          <>
            <PitchDivider className="my-14" />

            <h3 className="font-display text-3xl tracking-tight text-ink">Courts &amp; prices</h3>
            <p className="mt-3 text-sm leading-relaxed text-ink-dim">
              The hourly base price per court. Peak windows from your pricing rules
              still apply on top.
            </p>

            <div className="mt-8 space-y-4">
              {courts.length === 0 && (
                <p className="border border-dashed border-hairline-2 px-4 py-6 text-center font-mono text-[0.62rem] uppercase tracking-editorial text-ink-faint">
                  No courts yet — add your first one below
                </p>
              )}
              {courts.map((c) => (
                <CourtRow key={c.id} court={c} />
              ))}
            </div>

            <form onSubmit={handleAddCourt} className="mt-10 space-y-6">
              <p className="eyebrow">Add a court</p>
              <div className="grid gap-6 sm:grid-cols-3">
                <div>
                  <label htmlFor="ct-label" className={labelClass}>Label</label>
                  <input
                    id="ct-label"
                    value={courtLabel}
                    onChange={(e) => setCourtLabel(e.target.value)}
                    placeholder="Court A"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label htmlFor="ct-side" className={labelClass}>Side count</label>
                  <input
                    id="ct-side"
                    type="number"
                    min={3}
                    max={11}
                    value={courtSide}
                    onChange={(e) => setCourtSide(e.target.value)}
                    className={inputClass}
                  />
                </div>
                <div>
                  <label htmlFor="ct-price" className={labelClass}>Price (Rs./hr)</label>
                  <input
                    id="ct-price"
                    type="number"
                    min={100}
                    step={50}
                    value={courtPrice}
                    onChange={(e) => setCourtPrice(e.target.value)}
                    className={inputClass}
                  />
                </div>
              </div>
              <button type="submit" disabled={courtPending} className={buttonClass}>
                {courtPending ? "Adding…" : "Add court"}
              </button>
              <Feedback error={courtError} notice={null} />
            </form>
          </>
        )}
      </div>
    </section>
  );
}
