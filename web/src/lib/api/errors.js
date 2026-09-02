/**
 * The one error type every page catches, whichever transport produced it.
 *
 * It lives in its own module rather than beside the transport that throws it,
 * so that a second transport — a test double, or whatever replaces client.js —
 * can throw the same type without either importing the other.
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
