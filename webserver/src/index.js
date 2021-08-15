import React, { useState, useEffect, useContext, useRef } from 'react';
import ReactDOM from 'react-dom';

import Alert from 'react-bootstrap/Alert'
import Col from 'react-bootstrap/Col';
import Container from 'react-bootstrap/Container';
import Navbar from 'react-bootstrap/Navbar'
import Row from 'react-bootstrap/Row';
import Tabs from 'react-bootstrap/Tabs';
import Tab from 'react-bootstrap/Tab';

import BakinDashboard from './dashboard.js'
import DelegateRegister from './delegateregister.js'
import Settings from './settings'
import SetupWizard from './wizards'
import Voting from './voting.js'

import ToasterContext, { ToasterContextProvider } from './toaster.js';
import { BASE_URL, NO_SIGNER, NOT_REGISTERED, apiRequest } from './util.js';

import '../node_modules/bootstrap/dist/css/bootstrap.min.css';
import './index.css';

import logo from './logo512.png';


const Bakinbacon = () => {

	const [ delegate, setDelegate ] = useState("");
	const [ status, setStatus ] = useState({});
	const [ lastUpdate, setLastUpdate ] = useState(new Date().toLocaleTimeString());
	const [ connOk, setConnOk ] = useState(false);
	const [ isLoading, setIsLoading ] = useState(true);
	const [ inWizard, setInWizard ] = useState(false);

	const addToast = useContext(ToasterContext);

	// Hold a reference so we can cancel it externally
	const fetchStatusTimer = useRef();

	// On component load
	useEffect(() => {
		setIsLoading(true);
		fetchStatus();

		// Update every 10 seconds
		const idTimer = setInterval(() => fetchStatus(), 10000);
		fetchStatusTimer.current = idTimer;
		return () => {
			// componentWillUnmount()
			clearInterval(fetchStatusTimer.current);
		};
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [fetchStatusTimer]);
	
	// Update the state of being in the wizard from within the wizard
	const didEnterWizard = (wizType) => {
		setInWizard(wizType);
		clearInterval(fetchStatusTimer);
	}

	const didEnterRegistration = () => {
		// If we need to register as baker, stop fetching /api/status until that completes
		clearInterval(fetchStatusTimer);
	}

	const fetchStatus = () => {

		const statusApiUrl = BASE_URL + "/api/status";

		apiRequest(statusApiUrl)
		.then((statusRes) => {
			setDelegate(statusRes.pkh);
			setStatus(statusRes);
			setLastUpdate(new Date(statusRes.ts * 1000).toLocaleTimeString());
			setConnOk(true);
			setIsLoading(false);
		})
		.catch((errMsg) => {
			console.log(errMsg)
			setConnOk(false);
			addToast({
				title: "Fetch Dashboard Error",
				msg: "Unable to fetch status from BakinBacon ("+errMsg+"). Is the server running?",
				type: "danger",
				autohide: 10000,
			});
		})
	}

	// Returns
	if (!isLoading && ((!delegate && status.state === NO_SIGNER) || inWizard)) {
		// Need to run setup wizard
		return (
			<>
			<Container>
				<Row>
				  <Col>
					<Navbar bg="light">
						<Navbar.Brand><img src={logo} width="55" height="45" alt="BakinBacon Logo" />{' '}Bakin'Bacon</Navbar.Brand>
					</Navbar>
				  </Col>
				</Row>
				<Row>
					<SetupWizard didEnterWizard={didEnterWizard} />
				</Row>
			</Container>
			</>
		);
	}

	// Done loading; Display
	return (
		<>
		<Container>
			<Row>
			  <Col>
				<Navbar bg="light">
					<Navbar.Brand><img src={logo} width="55" height="45" alt="BakinBacon Logo" />{' '}Bakin'Bacon</Navbar.Brand>
					<Navbar.Collapse className="justify-content-end">
						<Navbar.Text>{delegate}</Navbar.Text>
					</Navbar.Collapse>
				</Navbar>
			  </Col>
			</Row>
			{ isLoading ? <Row><Col>Loading dashboard...</Col></Row> : 
			<Row>
			  <Col>
				<Tabs defaultActiveKey="dashboard" id="bakinbacon-tabs" mountOnEnter={true} unmountOnExit={true}>
					<Tab eventKey="dashboard" title="Dashboard">
						{ status.state === NOT_REGISTERED ?
						<DelegateRegister delegate={delegate} didEnterRegistration={didEnterRegistration} />
						:
						<BakinDashboard delegate={delegate} status={status} />
						}
					</Tab>
					<Tab eventKey="settings" title="Settings">
						<Settings />
					</Tab>
					<Tab eventKey="voting" title="Voting">
						<Voting delegate={delegate} />
					</Tab>
				</Tabs>
			  </Col>
			</Row>
			}
			<Row>
			  <Col>
				<Alert variant="secondary">
					<div className={"baconstatus baconstatus-" + (connOk ? "green" : "red") }></div>Last Update: {lastUpdate}
				</Alert>
			  </Col>
			</Row>
		</Container>
		</>
	);
}

ReactDOM.render(<ToasterContextProvider><Bakinbacon /></ToasterContextProvider>, document.getElementById('bakinbacon'));
