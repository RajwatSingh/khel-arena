/**
 * The one error type every page catches, whichever transport produced it.
 *
 * It lives in its own module because both `mock.js` and `client.js` throw it.
 * When it was defined in the mock, the real client had to import from the fake
 * one to construct an error — a dependency pointing exactly the wrong way, and
 * one that would have kept the mock in the production bundle after the
 * switchover.
 *
 * The shape mirrors what `domain.Error` reaches the wire as (apiPlan.md §4):
 * a machine-readable `code`, a message written for a person, and `fields` for
 * the per-input messages a form renders next to each control.
 */
export class ApiError extends Error {
	constructor(code, message, fields = []) {
		super(message);
		this.name = 'ApiError';
		this.code = code;
		this.fields = fields;
	}
}
