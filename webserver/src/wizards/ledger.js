import React, { useState } from 'react';

import Alert from 'react-bootstrap/Alert';
import Button from 'react-bootstrap/Button';
import Card from 'react-bootstrap/Card';
import Col from 'react-bootstrap/Col';
import Loader from "react-loader-spinner";
import Row from 'react-bootstrap/Row';

import { BaconAlert, apiRequest } from '../util.js';

import "react-loader-spinner/dist/loader/css/react-spinner-loader.css";


const WizardLedger = (props) => {

	const { onFinishWizard } = props;

	const [ step, setStep ] = useState(1)
	const [ alert, setAlert ] = useState({})
	const [ info, setInfo ] = useState({})
	const [ isLoading, setIsLoading ] = useState(false)

	const testLedger = () => {
		// Make API call to UI so BB can check for ledger

		// Clear previous errors
		setAlert({});
		setInfo({});
		setIsLoading(true);

		const testLedgerApiUrl = window.BASE_URL + "/api/wizard/testLedger";
		apiRequest(testLedgerApiUrl)
		.then((data) => {
			// Ledger and baking app detected by BB; enable continue button
			console.log(data);
			setInfo(data);
			setAlert({
				type: "success",
				msg: "Detected ledger: " + data.version
			});
			setStep(11);
		})
		.catch((errMsg) => {
			console.log(errMsg);
			setAlert({
				type: "danger",
				msg: errMsg,
			});
			setStep(1);
		})
		.finally(() => {
			setIsLoading(false);
		});
	}

	const stepTwo = () => {
		setAlert({
			type: "success",
			msg: "Baking Address: " + info.pkh
		});
		setStep(2);
	}

	const confirmBakingPkh = () => {

		// Still on step 2
		setIsLoading(true);

		const confirmPkhApiURL = window.BASE_URL + "/api/wizard/confirmBakingPkh"
		const requestOptions = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				bp: info.bipPath,
				pkh: info.pkh
			})
		};

		apiRequest(confirmPkhApiURL, requestOptions)
		.then((data) => {
			console.log(data);
			setAlert({
				type: "success",
				msg: "Yay! Baking address, " + info.pkh + ", confirmed!"
			});
			setStep(21);
		})
		.catch((errMsg) => {
			setAlert({
				type: "danger",
				msg: errMsg,
			});
		})
		.finally(() => {
			setIsLoading(false);
		});
	}

	// This renders inside parent <Card.Body>
	if (step === 1 || step === 11) {
		return (
			<>
			<Card.Title>Setup Ledger Device - Step 1</Card.Title>
			<Row>
				<Col md={{ span: 10, offset: 1 }}>
					<Card.Text>Please make sure that you have completed the following steps before continuing in Bakin&#39;Bacon:</Card.Text>
					<ol>
					  <li>Ensure Ledger device is plugged in to USB port on computer running Bakin&#39;Bacon.</li>
					  <li>Make sure Ledger is unlocked.</li>
					  <li>Install the 'Tezos Baking' application, and ensure it is open.</li>
					</ol>
					<Card.Text>If you do not have the 'Tezos Baking' application installed, you will need to download <a href="https://www.ledger.com/ledger-live/download" target="_blank" rel="noreferrer">Ledger Live</a> and use it to install the applications onto your device.</Card.Text>
					<Card.Text>You must successfully test your ledger before continuing. Please click the 'Test Ledger' button below.</Card.Text>
				</Col>
			</Row>
			<Row className="justify-content-md-center">
				<Col md={4}><Button variant="info" size="lg" block onClick={testLedger}>Test Ledger</Button></Col>
				<Col md={4}><Button disabled={step !== 11} variant={step === 11 ? "success" : "dark"} size="lg" block onClick={stepTwo}>Continue...</Button></Col>
			</Row>

			{ isLoading &&
			<Row className="justify-content-md-center">
			  <Col><Loader type="Circles" color="#EFC700" height={25} width={25} />Checking for Ledger...</Col>
			</Row>
			}

			<BaconAlert alert={alert} />
			</>
		);
	}
	
	if (step === 2 || step === 21) {
		return (
			<>
			<Card.Title>Setup Ledger Device - Step 2</Card.Title>
			<Row>
				<Col md={{ span: 10, offset: 1 }}>
					<Card.Text>Bakin&#39;Bacon has fetched the key shown below from the ledger device. This is the address that will be used for baking.</Card.Text>
					<Card.Text>You need to confirm this address by clicking the 'Confirm Address' button below, then look at your ledger device, compare the address displayed on the device to the address below, and then click the button on the device to confirm they match.</Card.Text>
					<Card.Text>After you confirm the addresses match, you can then click the "Let&#39;s Bake!" button.</Card.Text>
				</Col>
			</Row>
			<Row className="justify-content-md-center">
				<Col md={4}><Button variant="info" size="lg" block onClick={confirmBakingPkh}>Confirm Address</Button></Col>
				<Col md={4}><Button disabled={step !== 21} variant={step === 21 ? "success" : "dark"} size="lg" block onClick={onFinishWizard}>Yes! Let&#39;s Bake!</Button></Col>
			</Row>

			{ isLoading && <>
			<Row className="justify-content-md-center">
				<Col md="auto"><Loader type="Circles" color="#EFC700" height={25} width={25} /></Col>
			</Row>
			<Row className="justify-content-md-center">
				<Col md="auto">Waiting on user... Look at your ledger!</Col>
			</Row> </>
			}

			<BaconAlert alert={alert} />
			</>
		)
	}

	// Default shows error
	return (
		<Alert variant="danger">Uh oh... something went wrong. You should refresh your browser and start over.</Alert>
	);
}

export default WizardLedger
