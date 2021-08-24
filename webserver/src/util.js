// Utility functions and constants for BakinBacon

import Alert from 'react-bootstrap/Alert'
import Col from 'react-bootstrap/Col';
import Row from 'react-bootstrap/Row';

import { FaCheckCircle, FaExclamationTriangle } from "react-icons/fa";

export const LOW_BALANCE = "lowbal"
export const NO_SIGNER = "nosign"
export const CAN_BAKE = "canbake"
export const NOT_REGISTERED = "noreg"

export const BASE_URL = "http://10.10.10.203:8082"

export const CHAINID_MAINNET     = "NetXdQprcVkpaWU";
export const CHAINID_FLORENCENET = "NetXxkAx4woPLyu";
export const CHAINID_GRANADANET  = "NetXz969SFaFn8k";

export const MIN_BLOCK_TIME = 30;

// Copied from https://github.com/github/fetch/issues/203#issuecomment-266034180
function parseJSON(response) {

	// No JSON to parse, return custom object
	if (response.status !== 200 && response.status !== 400 && response.status !== 502) {
		return new Promise((resolve) => resolve({
			status: response.status,
			ok: response.ok,
			json: {"error": "Error Fetching URL"}
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
				return reject(response.json.error);
			})
			.catch((error) => reject(error.message));	// Network errors
	});
}

export const BaconAlert = (props) => {

	const { alert } = props;

	if (!alert.msg) {
		return null;
	}

	return (
		<Row>
		<Col>
		<Alert variant={alert.type}>
		{ alert.type === "success" &&
			<FaCheckCircle className='ledgerAlert' />
		}
		{ alert.type === "danger" &&
			<FaExclamationTriangle className='ledgerAlert' />
		}
		{alert.msg}
		{ alert.debug &&
		<>
		<br/>Error: {alert.debug}
		</>
		}
		</Alert>
		</Col>
		</Row>
	)
}

export const substr = (g) => {
	return String(g).substring(0, 10)
}
