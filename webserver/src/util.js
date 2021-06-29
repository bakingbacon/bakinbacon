// Utility functions for BakinBacon

// Copied from https://github.com/github/fetch/issues/203#issuecomment-266034180
function parseJSON(response) {

	// No JSON to parse, return custom object
	if (response.status !== 200 && response.status !== 400) {
		return new Promise((resolve) => resolve({
			status: response.status,
			ok: response.ok,
			message: "Error Fetching URL"
		}));
	}

	// Handles 200 OK and custom 400 JSON-encoded errors from API
	return new Promise((resolve) => response.json()
		.then((json) => resolve({
			status: response.status,
			ok: response.ok,
			json,
		}))
	);
}

export function apiRequest(url, options) {
	return new Promise((resolve, reject) => {
		fetch(url, options)
			.then(parseJSON)
			.then((response) => {
				// If 200 OK, resolve with JSON
				if (response.ok) {
					return resolve(response.json);
				}
				// Extract custom error message
				return reject(response.json.err);
			})
			.catch((error) => reject(error.message));	// Network errors
	});
}
