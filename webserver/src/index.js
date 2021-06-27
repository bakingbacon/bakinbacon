import React, { useState, useEffect, useContext } from 'react';
import ReactDOM from 'react-dom';

import Alert from 'react-bootstrap/Alert'
import Card from 'react-bootstrap/Card';
import Col from 'react-bootstrap/Col';
import Container from 'react-bootstrap/Container';
import Navbar from 'react-bootstrap/Navbar'
import Row from 'react-bootstrap/Row';
import Tabs from 'react-bootstrap/Tabs';
import Tab from 'react-bootstrap/Tab';

import BakinDashboard from './dashboard.js'
import Settings from './settings.js'
import SetupWizard from './setupwizard.js'

import ToasterContext, { ToasterContextProvider } from './toaster.js';

import '../node_modules/bootstrap/dist/css/bootstrap.min.css';
import './index.css';

import logo from './logo512.png';

//const BASE_URL = ""
const BASE_URL = "http://10.10.10.203:8082"

const Bakinbacon = () => {

	const [ delegate, setDelegate ] = useState("");
	const [ status, setStatus ] = useState({});
	const [ lastUpdate, setLastUpdate ] = useState(new Date().toLocaleTimeString());
	const [ connOk, setConnOk ] = useState(false);
	const [ isLoading, setIsLoading ] = useState(true);
	const [ inWizard, setInWizard ] = useState(false);

	const addToast = useContext(ToasterContext);

	useEffect(() => {
		setIsLoading(true);
		fetchStatus();

		// Update every 10 seconds
		let fetchStatusTimer = setInterval(() => fetchStatus(), 10000);
		return () => {
			// componentWillUnmount()
			clearInterval(fetchStatusTimer);
			fetchStatusTimer = null;
		};
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);
	
	// Update the state of being in the wizard from within the wizard
	const didEnterWizard = (wizType) => {
		setInWizard(wizType);
	}

	const fetchStatus = () => {
		const delegateApiUrl = BASE_URL + "/api/delegate";
		const statusApiUrl = BASE_URL + "/api/status";

		Promise.all([fetch(delegateApiUrl), fetch(statusApiUrl)])
		.then(([delegateResp, statusResp]) => {
			return Promise.all([delegateResp.json(), statusResp.json()])
		})
		.then(([delegateRes, statusRes]) => {
			setDelegate(delegateRes.pkh);
			setStatus(statusRes);
			setLastUpdate(new Date(statusRes.ts * 1000).toLocaleTimeString());
			setConnOk(true);
			setIsLoading(false);
		})
		.catch((e) => {
			console.log(e)
			setConnOk(false);
			addToast({
				title: "Fetch Status Error",
				msg: "Unable to fetch status from BakinBacon ("+e.message+"). Is the server running?",
				type: "danger",
				autohide: 10000,
			});
		})
	}

	// Returns
	if (!isLoading && (!delegate || inWizard)) {
		// Need to run setup wizard
		return (
			<Container>
				<Navbar bg="light">
					<Navbar.Brand><img src={logo} width="55" height="45" alt="BakinBacon Logo" />{' '}Bakin'Bacon</Navbar.Brand>
				</Navbar>
				<br />
				<SetupWizard didEnterWizard={didEnterWizard} />
			</Container>
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
						<BakinDashboard delegate={delegate} status={status} />
					</Tab>
					<Tab eventKey="settings" title="Settings">
						<Settings status={status} />
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
