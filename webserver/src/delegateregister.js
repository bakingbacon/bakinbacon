import React, { useState, useContext, useEffect } from 'react';

import Alert from 'react-bootstrap/Alert';
import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Loader from "react-loader-spinner";
import Row from 'react-bootstrap/Row';

import ToasterContext from './toaster.js';
import { BASE_URL, BaconAlert, apiRequest } from './util.js';

import "react-loader-spinner/dist/loader/css/react-spinner-loader.css";


const DelegateRegister = (props) => {

	const { delegate, didEnterRegister } = props;

	const [ step, setStep ] = useState(0);
	const [ alert, setAlert ] = useState({})
	const [ isLoading, setIsLoading ] = useState(false);
	const [ balance, setBalance ] = useState(0);
	const addToast = useContext(ToasterContext);

	useEffect(() => {

		didEnterRegister();  // Tell parent we are in here

		// If not registered, fetch balance every 5min
		fetchBalanceInfo();

		let fetchBalanceInfoTimer = setInterval(() => fetchBalanceInfo(), 1000 * 60 * 5);
		return () => {
			// componentWillUnmount()
			clearInterval(fetchBalanceInfoTimer);
			fetchBalanceInfoTimer = null;
		};
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const registerBaker = () => {
		const registerBakerApiUrl = BASE_URL + "/api/wizard/registerBaker";
		const requestOptions = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
		};

		setIsLoading(true);

		apiRequest(registerBakerApiUrl, requestOptions)
		.then((data) => {
			console.log("Register OpHash: " + data.ophash);
			setIsLoading(false);
			setStep(99);
		})
		.catch((errMsg) => {
			console.log(errMsg)
			setIsLoading(false);
			setAlert({
				type: "danger",
				msg: errMsg,
			});
		});
	};

	// If baker is not yet revealed/registered, we need to monitor basic
	// balance so we can display the button when enough funds are available.
	// Check every 5 minutes
	const fetchBalanceInfo = () => {

		setIsLoading(true);

		const balanceUrl = "http://florencenet-us.rpc.bakinbacon.io/chains/main/blocks/head/context/contracts/" + delegate
		apiRequest(balanceUrl)
		.then((data) => {
			setBalance((parseInt(data.balance, 10) / 1e6).toFixed(1));
		})
		.catch((errMsg) => {
			console.log(errMsg)
			addToast({
				title: "Loading Balance Error",
				msg: errMsg,
				type: "danger",
			});
		})
		.finally(() => {
			setIsLoading(false);
		})
	}

	// Returns
	if (step === 99) {
		return (
			<Card>
				<Card.Header as="h5">Baker Status</Card.Header>
				<Card.Body>
					<Alert variant="success">Your baking address, {delegate}, has been registered as a baker!</Alert>
					<Card.Text>It is now time to wait, unfortunately. In order to protect against bakers coming and going, the Tezos network will not include your registration for 3 cycles.
					After that waiting period, you will begin to receive baking and endorsing opportunities for future cycles.
					BakinBacon will always attempt to inject every endorsement you are granted, and only considers priority 0 baking opportunities.</Card.Text>
					<Card.Text>Reload this page to view your baker stats, such as staking balance, and number of delegators. You will also be able to view your next baking and endorsing opportunities when they are granted by the network.</Card.Text>
				</Card.Body>
			</Card>
		)
	}
	
	// default
	return (
		<Card>
			<Card.Header as="h5">Baker Status</Card.Header>
			<Card.Body>
			{ isLoading ? <>
				<Row className="justify-content-md-center">
					<Col md="auto"><Loader type="Circles" color="#EFC700" height={25} width={25} /></Col>
				</Row>
				<Row className="justify-content-md-center">
					<Col md="auto">Submitting baker registration to network. This may take up to 5 minutes to process. Please wait. This page will automatically update when registration has been submitted.</Col>
				</Row>
				<Row className="justify-content-md-center">
					<Col md="auto"><em>If you are using a ledger device, please look at the device and approve the registration</em>.</Col>
				</Row> </>
				:
				<>
				<Card.Text>Your baking address, {delegate}, has not been registered as a baker to the Tezos network. In order to be a baker, you need to have at least 8000 tez in your baking address.
				A small, one-time fee of 0.257 XTZ, is also required to register, in addition to standard operation fees. 1 additional tez will cover this.</Card.Text>
				<Card.Text>There is currently {balance} XTZ in your baking address.</Card.Text>

				{ balance < 8001 ?
				  <Card.Text>Please ensure your balance is at least 8001 XTZ so that we can complete the registration process.</Card.Text>
				:
				<>
				<Row>
					<Col>If you are using a ledger device, you will be prompted to confirm this action. Please ensure your device is unlocked and the <b>Tezos Baking</b> application is loaded.</Col>
				</Row>
				<Row>
					<Col md={{span: 8, offset: 2}}><Button variant="primary" size="lg" block onClick={registerBaker}>Register My Address As Baker!</Button></Col>
				</Row>
				</>
				}
				</>
			}
			<BaconAlert alert={alert} />
			</Card.Body>
		</Card>
	)
}

export default DelegateRegister