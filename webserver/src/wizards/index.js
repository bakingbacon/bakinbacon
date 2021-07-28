import React, { useState } from 'react';

import Alert from 'react-bootstrap/Alert'
import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Row from 'react-bootstrap/Row';

import WizardWallet from './wallet.js';
import WizardLedger from './ledger.js';

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
	)
}

export default SetupWizard
