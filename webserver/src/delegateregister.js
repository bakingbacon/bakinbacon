import React, { useState, useContext } from 'react';

import Alert from 'react-bootstrap/Alert'
import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Row from 'react-bootstrap/Row';

import ToasterContext from './toaster.js';

//const BASE_URL = ""
const BASE_URL = "http://10.10.10.203:8082"

const DelegateRegister = (props) => {

	const addToast = useContext(ToasterContext);

	const [ step, setStep ] = useState(0);
	const [ isLoading, setIsLoading ] = useState(false);
	// const [ opHash, setOphash ] = useState("")
	
	const registerBaker = () => {
		const registerBakerApiUrl = BASE_URL + "/api/wizard/registerbaker";
		const requestMetadata = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
		};

		setIsLoading(true);

		fetch(registerBakerApiUrl, requestMetadata)
		.then(response => {
			if (!response.ok) {
				throw Error(response.statusText);
			}
			return response.json();
		}).then(data => {
			console.log("Register OpHash: " + data.ophash);
			setIsLoading(false);
			setStep(99);
		})
		.catch(e => {
			console.log(e)
			setIsLoading(false);
			addToast({
				title: "Register Baker Error",
				msg: e.message,
				type: "danger",
			});
		});
	}

	// Returns
	if (step === 99) {
		return (
			<>
			<Row>
				<Col>
					<Card>
						<Card.Header as="h5">Baker Status</Card.Header>
						<Card.Body>
							<Alert variant="success">Your baking address, {props.pkh}, has been registered as a baker!</Alert>
							<Card.Text>It is now time to wait, unfortunately. In order to protect against bakers coming and going, the Tezos network will not include your registration for 3 cycles.
							After that waiting period, you will begin to receive baking and endorsing opportunities for future cycles.
							BakinBacon will always attempt to inject every endorsement you are granted, and only considers priority 0 baking opportunities.</Card.Text>
							<Card.Text>Reload this page to view your baker stats, such as staking balance, and number of delegators. You will also be able to view your next baking and endorsing opportunities when they are granted by the network.</Card.Text>
						</Card.Body>
					</Card>
				</Col>
			</Row>
			</>
		)
	}
	
	// else
	const canRegister = props.spendable >= 8001
	const curBalance = parseInt(props.spendable, 10).toFixed(0)
	
	return (
		<>
		<Row>
			<Col>
				<Card>
					<Card.Header as="h5">Baker Status</Card.Header>
					<Card.Body>
					{ isLoading && 
						<Card.Text>Submitting baker registration to network. This may take up to 5 minutes to process. Please wait. This page will automatically refresh when registration is complete.</Card.Text>
					} else {
						<>
						<Card.Text>Your baking address, {props.pkh}, has not been registered as a baker to the Tezos network. In order to be a baker, you need to have at least 8000 Tez in your baking address.
						A small, one-time fee of 0.257 XTZ, is also required to register, in addition to standard operation fees. 1 additional Tez will cover this.</Card.Text>
						<Card.Text>There is currently {curBalance} XTZ in your baking address.</Card.Text>

						{ !canRegister &&
						  <Card.Text>Please ensure your balance is at least 8001 XTZ so that we can complete the registration process.</Card.Text>
						} else {
						<>
						<Row>
							<Col md={{span: 8, offset: 2}}><Button variant="primary" size="lg" block onClick={registerBaker}>Register My Address As Baker!</Button></Col>
						</Row>
						<Row>
							<Col>If you are using a ledger device, you will be prompted to confirm this action. Please ensure your device is unlocked and the <b>Tezos Baking</b> application is loaded.</Col>
						</Row>
						</>
						}
						</>
					}
					</Card.Body>
				</Card>
			</Col>
		</Row>
		</>			
	)
}

export default DelegateRegister