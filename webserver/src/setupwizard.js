import React, { useState } from 'react';

import Alert from 'react-bootstrap/Alert'
import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form'
import Row from 'react-bootstrap/Row';

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
		fetch(generateKeyApiUrl)
		.then(response => {
			if (!response.ok) {
				throw Error(response.statusText);
			}
			return response.json();
		})
		.then(data => {
			setEdsk(data.edsk);
			setPkh(data.pkh);
			setStep(2);
		})
		.catch(e => {
			console.log(e)
			setError(e.message)
		});
	};
	
	const exitWizardWallet = () => {
		const finishWizardApiUrl = BASE_URL + "/api/wizard/finish";
		fetch(finishWizardApiUrl)
		.then(response => {
			if (!response.ok) {
				throw Error(response.statusText);
			}
			return response;
		})
		.then(data => {
			// Ignore response body; just need 200 OK
			// Call parent finish wizard to exit this sub-wizard
			onFinishWizard();
		})
		.catch(e => {
			console.log(e);
			setError(e.message)
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
		const requestMetadata = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ edsk: importEdsk })
		};

		fetch(importKeyApiUrl, requestMetadata)
		.then(response => {
			if (!response.ok) {
				throw Error(response.statusText);
			}
			return response.json();
		})
		.then(data => {
			setEdsk(data.edsk);
			setPkh(data.pkh);
			setStep(3);
		})
		.catch(e => {
			console.log(e);
			setError(e.message)
		});
	}
	
	// Returns

	// Step 99 is a dummy step that happens last
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

const WizardLedger = (props) => {

	const { onFinishWizard } = props;

	const [ step, setStep ] = useState(1)
	
	// This renders inside parent <Card.Body>
	if (step === 1) {
		return(
			<Card.Text>
			<p>This is ledger wizard step 1</p>
			</Card.Text>
		);
	}
	
	// Default shows error
	return (
		<Button variant="warning" block onClick={onFinishWizard}>Uh oh... something went wrong.</Button>
	);
}

// --
// -- Main Wizard Class
// --

const SetupWizard = (props) => {

	const { didEnterWizard } = props;

	const [ wizardType, setWizardtype ] = useState("");

	const selectWizard = (opt) => {
		didEnterWizard(true);  // Tells parent component to continue displaying wizard even after delegate is set
		setWizardtype(opt);
	}
	
	const finishWizard = () => {
		setWizardtype("fin");
	}
	
	return (
		<Row>
			<Col md="12">
				<Card>
				  <Card.Header as="h5">Setup Wizard</Card.Header>
				  <Card.Body>

					{ !wizardType &&
					  <>
					  <Card.Title>Welcome to Bakin'Bacon!</Card.Title>
					  <Card.Text>It appears that you have not configured Bakin'Bacon, so let's do that now.</Card.Text>
					  <Card.Text>You first need to decide where to store your super-secret private key using for baking on the Tezos blockchain. You have two choices, listed below, along with some pros/cons for each option:</Card.Text>

					  <ul>
					   <li>Software Wallet
						<ul>
						 <li>Pro: Built-in; No external hardware</li>
						 <li>Pro: Can export private key for backup</li>
						 <li>Con: Not as secure as hardware-based solutions</li>
						</ul>
					   </li>
					   <li>Ledger Device
						<ul>
						 <li>Pro: Ultra-secure device, proven in the industry</li>
						 <li>Pro: Physical confirmation required for any transaction</li>
						 <li>Con: External hardware component creates additional dependencies</li>
						</ul>
					   </li>
					  </ul>

					  <Card.Text><b>We highly recommend the use of a ledger device for maximum security.</b></Card.Text>

					  <Card.Text>Please select your choice by clicking on one of the buttons below:</Card.Text>
					
					  <Alert variant="warning"><strong>WARNING:</strong> This choice is <em>permanent</em>! If you pick software wallet now, you <strong>cannot</strong> switch to ledger in the future, as ledger does not support importing keys. Similarly, if you pick Ledger now you <strong>cannot</strong> switch to software wallet, as ledger does not allow you to export keys.</Alert>
				
					  <Row>
					   <Col md="6"><Button variant="primary" size="lg" block onClick={() => selectWizard("wallet")}>Software Wallet</Button></Col>
					   <Col md="6"><Button variant="primary" size="lg" block onClick={() => selectWizard("ledger")}>Ledger Wallet</Button></Col>
					  </Row>

					  </>
					}
					
					{ wizardType === "wallet" && <WizardWallet onFinishWizard={finishWizard} /> }
					{ wizardType === "ledger" && <WizardLedger onFinishWizard={finishWizard} /> }
					
					{ wizardType === "fin" &&
						<>
						<Card.Title>Setup Complete</Card.Title>
						<Card.Text>Congratulations! You have set up Bakin'Bacon.</Card.Text>
						<Card.Text>Now that you have an address for use on the Tezos blockchain, you will need to fund this address with a minimum of 8,001 XTZ in order to become a baker.</Card.Text>
						<Card.Text>For every 8,000 XTZ in your adress, the network grants you 1 roll. In simplistic terms, at the start of every cycle, the blockchain determines how many rolls each baker has and randomly assigns baking rights based on how many each baker has. The more rolls you have, the more chances you have to earn baking and endorsing rights.</Card.Text>
						<Card.Text>There is no guarantee you will get rights every cycle. It is pure random chance. This is one aspect that makes Tezos hard to take advantage of by malicious attackers.</Card.Text>
						<Card.Text>You can refresh this page to see your status.</Card.Text>
						</>
					}
					  
				  </Card.Body>
				</Card>
			</Col>
		</Row>
	)
}

export default SetupWizard
