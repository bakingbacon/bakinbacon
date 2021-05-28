import React from 'react';

import Alert from 'react-bootstrap/Alert'
import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form'
import Row from 'react-bootstrap/Row';

class WizardWallet extends React.Component {

	constructor(props) {
		super(props);
		
		this.state = {
			step: this.props.step || 0,
			err: "",
		}
		
		this.generateNewKey = this.generateNewKey.bind(this)
		this.doImportKey = this.doImportKey.bind(this)
		this.exitWizardWallet = this.exitWizardWallet.bind(this)
		this.selectWizardWallet = this.selectWizardWallet.bind(this)
		this.onSecretKeyChange = this.onSecretKeyChange.bind(this)
	};
	
	selectWizardWallet(e) {
		this.props.onSelectWizard('wallet');
		this.setState({step: 1});
	}
	
	generateNewKey(e) {
		const generateKeyApiUrl = "/api/wizard/generateNewKey";
		fetch(generateKeyApiUrl)
			.then(response => {
				if (!response.ok) {
					throw Error(response.statusText);
				}
				return response;
			})
			.then(response => response.json())
			.then(jRes => {
				this.setState({
					edsk: jRes.edsk,
					pkh: jRes.pkh,
					step: 2,
				});
			})
			.catch(error => {
				this.setState({
					err: error,
				});
				// TODO: Toaster
				console.log(error);
			});
	}
	
	exitWizardWallet(e) {
		const finishWizardApiUrl = "/api/wizard/finish";
		fetch(finishWizardApiUrl)
			.then(response => {
				if (!response.ok) {
					throw Error(response.statusText);
				}
				return response;
			})
			.then(response => {
				// Ignore response body; just need 200 OK
				this.props.onFinishWizard()
				this.setState({step: 99})
			})
			.catch(error => {
				this.setState({
					err: error
				});
				// TODO: Toaster
				console.log(error);
			});
	}
	
	onSecretKeyChange(e) {
		this.setState({
			importEdsk: e.target.value
		});
	}
	
	doImportKey(e) {
	
		// Clear previous error messages
		this.setState({err:""})
	
		// Sanity checks
		const edskInput = this.state.importEdsk
		if (edskInput.substring(0, 4) !== "edsk") {
			this.setState({
				err: "Secret key must begin with 'edsk'"
			})
			return
		}
		if (edskInput.length !== 54 && edskInput.length !== 98) {
			this.setState({
				err: "Secret key must be 54 or 98 characters long."
			})
			return
		}
	
		const importKeyApiUrl = "/api/wizard/importKey";
		const postBody = {
			edsk: this.state.importEdsk,
		};
		const requestMetadata = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(postBody)
		};

		fetch(importKeyApiUrl, requestMetadata)
			.then(response => {
				if (!response.ok) {
					throw Error(response.statusText);
				}
				return response;
			})
			.then(response => response.json())
			.then(jRes => {
				this.setState({
					edsk: jRes.edsk,
					pkh: jRes.pkh,
					step: 3,
				});
			})
			.catch(err => {
				// TODO
				console.log("Error: " + err)
			});
	}
	
	render() {
		const errMsg = this.state.err

		// Step 99 is a dummy step that happens last
		if (this.state.step === 99) { return (<>Foo</>); }

		// This renders inside parent <Card.Body>
		if (this.state.step === 1) {
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
					<Col md="4"><Button variant="primary" size="lg" block onClick={this.generateNewKey}>Generate New Key</Button></Col>
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
							<Form.Control type="text" placeholder="edsk..." onChange={this.onSecretKeyChange} />
						</Form.Group>
					</Col>
					<Col md="3" className="mt-3"><Button variant="primary" size="lg" block onClick={this.doImportKey}>Import Secret Key</Button></Col>
				</Row>

				{ errMsg &&
				<Alert variant="danger">{errMsg}</Alert>
				}
				</>
			);
		}
		
		// Successfully generated new key; display for user
		if (this.state.step === 2) {
			const edsk = this.state.edsk;
			const pkh = this.state.pkh;
			
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
					<Col md={3}><Button variant="primary" block onClick={this.exitWizardWallet}>I saved my key; Continue</Button></Col>
				</Row>
				</>
			);
		}
		
		// Successfully imported key
		if (this.state.step === 3) {
			const pkh = this.state.pkh;
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
					<Col md={3}><Button variant="primary" block onClick={this.exitWizardWallet}>Key is Correct; Continue</Button></Col>
				</Row>
				</>
			);
		}
		
		// Default step 0 shows button inside parent Row-Col
		return (
			<Button variant="primary" size="lg" block onClick={this.selectWizardWallet}>Software Wallet</Button>
		);
	}
}

class WizardLedger extends React.Component {

	constructor(props) {
		super(props);
		this.selectWizard = this.selectWizard.bind(this)
		this.state = {
			step: this.props.step || 0,
		}
	};
	
	selectWizard(e) {
		this.props.onSelectWizard('ledger');
		this.setState({step: 1});
	}
	
	render() {
	
		// This renders inside parent <Card.Body>
		if (this.state.step === 1) {
			return(
				<Card.Text>
				<p>This is ledger wizard step 1</p>
				</Card.Text>
			);
		}
		
		// Default step 0 shows button inside parent Row-Col
		return (
			<Button variant="primary" size="lg" block onClick={this.selectWizard}>Ledger Wallet</Button>
		);
	}
}

// --
// -- Main Wizard Class
// --

class SetupWizard extends React.Component {

	constructor(props) {
		super(props);

		this.state = {
			wizardType: "",
		}
		
		this.selectWizard = this.selectWizard.bind(this);
		this.finishWizard = this.finishWizard.bind(this);
	};
	
	componentDidMount() {
	}
	
	componentWillUnmount() {
	}
	
	selectWizard(opt) {
		this.props.didEnterWizard(true)  // Tells parent component to continue displaying wizard even after delegate is set
		this.setState({wizardType: opt})
	}
	
	finishWizard() {
		this.setState({wizardType: "fin"})
	}
	
	render() {
		const { wizardType } = this.state
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
						   <Col md="6"><WizardWallet step={0} onSelectWizard={this.selectWizard} /></Col>
						   <Col md="6"><WizardLedger step={0} onSelectWizard={this.selectWizard} /></Col>
						  </Row>

						  </>
						}
						
						{ wizardType === "wallet" && <WizardWallet step={1} onFinishWizard={this.finishWizard} /> }
						{ wizardType === "ledger" && <WizardLedger step={1} onFinishWizard={this.finishWizard} /> }
						
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
}

export default SetupWizard
