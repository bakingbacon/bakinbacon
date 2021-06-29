import React, { useState } from 'react';

import Alert from 'react-bootstrap/Alert'
import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form'
import Row from 'react-bootstrap/Row';

import { apiRequest } from '../util.js';

//const BASE_URL = ""
const BASE_URL = "http://10.10.10.203:8082"

const WizardWallet = (props) => {

	const { onFinishWizard } = props;

	const [ step, setStep ] = useState(1);
	const [ edsk, setEdsk ] = useState("");
	const [ importEdsk, setImportEdsk ] = useState("");
	const [ pkh, setPkh ] = useState("");
	const [ err, setError ] = useState("");
	
	const generateNewKey = () => {
		const generateKeyApiUrl = BASE_URL + "/api/wizard/generateNewKey";
		apiRequest(generateKeyApiUrl)
		.then((data) => {
			setEdsk(data.edsk);
			setPkh(data.pkh);
			setStep(2);
		})
		.catch((errMsg) => {
			console.log(errMsg)
			setError(errMsg)
		});
	};
	
	const exitWizardWallet = () => {
		const finishWizardApiUrl = BASE_URL + "/api/wizard/finish";
		apiRequest(finishWizardApiUrl)
		.then(() => {
			// Ignore response body; just need 200 OK
			// Call parent finish wizard to exit this sub-wizard
			onFinishWizard();
		})
		.catch((errMsg) => {
			console.log(errMsg);
			setError(errMsg)
		});
	}
	
	const onSecretKeyChange = (e) => {
		setImportEdsk(e.target.value);
	}
	
	const doImportKey = () => {
	
		// Clear previous error messages
		setError("");
	
		// Sanity checks
		if (importEdsk.substring(0, 4) !== "edsk") {
			setError("Secret key must begin with 'edsk'");
			return
		}
		if (importEdsk.length !== 54 && importEdsk.length !== 98) {
			setError("Secret key must be 54 or 98 characters long.");
			return
		}

		// Call API to import key
		const importKeyApiUrl = BASE_URL + "/api/wizard/importKey";
		const requestOptions = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ edsk: importEdsk })
		};

		apiRequest(importKeyApiUrl, requestOptions)
		.then((data) => {
			setEdsk(data.edsk);
			setPkh(data.pkh);
			setStep(3);
		})
		.catch((errMsg) => {
			console.log(errMsg);
			setError(errMsg)
		});
	}
	
	// Returns

	// Step 99 is a dummy step that should not ever get rendered
	if (step === 99) { return (<>Foo</>); }

	// This renders inside parent <Card.Body>
	if (step === 1) {
		return(
			<>
			<Card.Title>Setup Software Wallet</Card.Title>
			<Row>
				<Col>
					<Card.Text>There are two options when setting up a software wallet: 1) Generate a new secret key, or 2) Import an existing secret key.</Card.Text>
					<Card.Text>Below, make your selection by clicking on 'Generate New Key', or by pasting your existing secret key and clicking 'Import Secret Key'. Your secret key must be unencrypted when importing.</Card.Text>
				</Col>
			</Row>
			<Row className="justify-content-md-center">
				<Col md="4"><Button variant="primary" size="lg" block onClick={generateNewKey}>Generate New Key</Button></Col>
			</Row>
			<Row className="justify-content-md-center">
				<Col><Form.Control plaintext readOnly id="generatedKey" /></Col>
			</Row>
			<Row className="justify-content-md-center">
				<Col md={{span: 3}}><hr/></Col><Col md="1" className="text-center">OR</Col><Col md={{span: 3}}><hr/></Col>
			</Row>
			<Row className="justify-content-md-center">
				<Col md="7">
					<Form.Group controlId="exampleForm.ControlInput1">
						<Form.Label>Secret Key</Form.Label>
						<Form.Control type="text" placeholder="edsk..." onChange={onSecretKeyChange} />
					</Form.Group>
				</Col>
				<Col md="3" className="mt-3"><Button variant="primary" size="lg" block onClick={doImportKey}>Import Secret Key</Button></Col>
			</Row>

			{ err &&
			<Alert variant="danger">{err}</Alert>
			}
			</>
		);
	}
	
	// Successfully generated new key; display for user
	if (step === 2) {
		return (
			<>
			<Card.Title>Setup Software Wallet</Card.Title>
			<Row className="justify-content-md-center">
				<Col>
					<Alert variant="success">Successfully generated new key!</Alert>
					<Card.Text>Below you will see your unencrypted secret key, along with your public key hash.</Card.Text>
					<Alert variant="warning">Save a copy of your secret key <b>NOW!</b> <em>It will never be displayed again.</em> Save it somewhere safe. In the future, if you need to restore Bakin'Bacon, you can import this key.</Alert>
				</Col>
			</Row>
			<Row>
				<Col md={2}><b>Secret Key:</b></Col>
				<Col>{edsk}</Col>
			</Row>
			<Row>
				<Col md={2}><b>Public Key Hash:</b></Col>
				<Col>{pkh}</Col>
			</Row>
			<Row>
				<Col md={3}><Button variant="primary" block onClick={exitWizardWallet}>I saved my key; Continue</Button></Col>
			</Row>
			</>
		);
	}
	
	// Successfully imported key
	if (step === 3) {
		return (
			<>
			<Card.Title>Setup Software Wallet</Card.Title>
			<Row className="justify-content-md-center">
				<Col>
					<Alert variant="success">Successfully imported secret key!</Alert>
					<Card.Text>Below you will see your public key hash. Confirm this is the correct address. If not, reload this page to try again.</Card.Text>
				</Col>
			</Row>
			<Row>
				<Col md={2}><b>Public Key Hash:</b></Col>
				<Col>{pkh}</Col>
			</Row>
			<Row>
				<Col md={3}><Button variant="primary" block onClick={exitWizardWallet}>Key is Correct; Continue</Button></Col>
			</Row>
			</>
		);
	}
}

export default WizardWallet
