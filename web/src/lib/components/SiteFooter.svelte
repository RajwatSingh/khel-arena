<script>
	import CentreMark from './CentreMark.svelte';
	import TypingAnimation from './TypingAnimation.svelte';
	import { reveal } from '$lib/actions/reveal.js';

	const madeBy = 'Made by Rajwat';
	const madeFor = 'Made for the game.';
	const typeSpeed = 38;
</script>

<footer use:reveal>
	<div class="shell">
		<div class="grid">
			<div class="about">
				<span class="mark">
					<CentreMark width={26} />
					Khel Arena
				</span>
				<p class="small">
					Court hours in Kathmandu and Lalitpur, bookable by the hour. Built on a database that
					will not let two games start on the same turf.
				</p>
			</div>

			<nav aria-label="Booking">
				<h2 class="label">Book</h2>
				<a class="foot-link" href="/tonight">Find a court</a>
				<a class="foot-link" href="/arenas">All arenas</a>
				<a class="foot-link" href="/bookings">My bookings</a>
			</nav>

			<nav aria-label="Playing">
				<h2 class="label">Play</h2>
				<a class="foot-link" href="/games">Games needing players</a>
				<a class="foot-link" href="/teams">My teams</a>
				<a class="foot-link" href="/tournaments">Tournaments</a>
				<a class="foot-link" href="/standings">The table</a>
			</nav>

			<nav aria-label="Account">
				<h2 class="label">Account</h2>
				<a class="foot-link" href="/login">Sign in</a>
				<a class="foot-link" href="/register">Create an account</a>
				<a class="foot-link" href="/manage">Manage a venue</a>
			</nav>

			<div>
				<h2 class="label">Paying</h2>
				<p class="small">eSewa, Khalti, or cash at the arena.</p>
				<p class="small dim">Prices are per hour, per court, in NPR. The arena sets them.</p>
			</div>
		</div>

		<div class="colophon">
			<p class="small">Kathmandu · UTC+05:45 · Khel Arena {new Date().getFullYear()}</p>
			<div class="tagline">
				<p class="small">
					<TypingAnimation text={madeFor} speed={typeSpeed} />
				</p>
				<p class="small">
					<TypingAnimation
						text={madeBy}
						speed={typeSpeed}
						startDelay={madeFor.length * typeSpeed + 320}
					/>
				</p>
			</div>
		</div>
	</div>
</footer>

<style>
	footer {
		background: var(--surface);
		border-top: 1px solid var(--line);
		padding-block: clamp(3rem, 6vw, 4.5rem) 2rem;
	}

	.grid {
		display: grid;
		grid-template-columns: 1.7fr repeat(3, 1fr);
		gap: 2.5rem 2rem;
	}

	.mark {
		display: inline-flex;
		align-items: center;
		gap: 0.6rem;
		font-family: var(--display);
		font-size: 1.25rem;
		font-weight: 500;
		letter-spacing: -0.02em;
	}

	.mark :global(svg) {
		color: var(--pine);
	}

	.about p {
		margin-top: 1rem;
		max-width: 38ch;
		color: var(--muted);
	}

	nav {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 0.5rem;
	}

	.label {
		margin-bottom: 0.35rem;
	}

	.foot-link {
		font-size: 0.9375rem;
		color: var(--muted);
		transition: color var(--dur-hover) var(--ease);
	}

	.foot-link:hover {
		color: var(--pine);
	}

	.small + .small {
		margin-top: 0.6rem;
	}

	.dim {
		color: var(--faint);
	}

	.colophon {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem 1.5rem;
		margin-top: 3rem;
		padding-top: 1.5rem;
		border-top: 1px solid var(--line);
		color: var(--faint);
	}

	.tagline {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
	}

	.tagline .small {
		font-family: var(--sans);
		font-size: 0.8125rem;
		font-weight: 700;
		font-style: normal;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--pine);
	}

	@media (max-width: 760px) {
		.grid {
			grid-template-columns: 1fr 1fr;
		}

		.about {
			grid-column: 1 / -1;
		}

		.tagline {
			align-items: flex-start;
		}
	}
</style>
