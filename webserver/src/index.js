import React from 'react';
import ReactDOM from 'react-dom';

import Container from 'react-bootstrap/Container';
import Col from 'react-bootstrap/Col';
import Row from 'react-bootstrap/Row';
import Card from 'react-bootstrap/Card';
import Navbar from 'react-bootstrap/Navbar'
import ProgressBar from 'react-bootstrap/ProgressBar';

import DelegateInfo from './delegateinfo.js'
import SetupWizard from './setupwizard.js'

import '../node_modules/bootstrap/dist/css/bootstrap.min.css';
import './index.css';

import logo from './BakinBacon_32x32.png';


class Bakinbacon extends React.Component {

	constructor(props) {
		super(props);
		
		this.state = {
			delegate: "",
			status: {},
			lastUpdate: "",
			connOk: false,
			isLoading: false,
			inWizard: false,
		};
		
		this.didEnterWizard = this.didEnterWizard.bind(this)
	}

	componentDidMount() {
		this.setState({ isLoading: true });
		this.fetchStatus();
		this.fetchStatusTimer = setInterval(() => this.fetchStatus(), 10000);
	}
	
	componentWillUnmount() {
		clearInterval(this.fetchStatusTimer);
		this.fetchStatusTimer = null;
	}
	
	// Update the state of being in the wizard from within the wizard
	didEnterWizard(s) {
		this.setState({
			inWizard: s,
		});
	}
	
	fetchStatus() {
		const delegateApiUrl = "http://10.10.10.203:8082/api/delegate";
		const statusApiUrl = "http://10.10.10.203:8082/api/status";

		Promise.all([
			fetch(delegateApiUrl),
			fetch(statusApiUrl)
		]).then(([delegateResp, statusResp]) => {
			return Promise.all([delegateResp.json(), statusResp.json()])
		}).then(([delegate, status]) => {
			var df = new Date(status.ts * 1000).toLocaleTimeString()
			this.setState({
				delegate: delegate.pkh,
				status: status,
				lastUpdate: df,
				connOk: true,
				isLoading: false,
			});
		}).catch((e) => {
			console.log(e)
			// TODO: Need toaster for errors
		})
	}

	render() {
		const { delegate, status, lastUpdate, connOk, isLoading, inWizard } = this.state

		if (isLoading) {
			return (<p>Loading...</p>)
		}

		if (!delegate || inWizard) {
			// Need to run setup wizard
			return (
				<Container>
					<Navbar bg="light">
						<Navbar.Brand><img src={logo} alt="BakinBacon Logo" />{' '}Bakin'Bacon</Navbar.Brand>
					</Navbar>
					<br />
					<SetupWizard didEnterWizard={this.didEnterWizard} />
				</Container>
			);
		}

		// Done loading; Display
		return (
			<Container>
				<Navbar bg="light">
					<Navbar.Brand><img src={logo} alt="BakinBacon Logo" />{' '}Bakin'Bacon</Navbar.Brand>
					<Navbar.Collapse className="justify-content-end">
						<Navbar.Text>{delegate}</Navbar.Text>
					</Navbar.Collapse>
				</Navbar>
				<br />
				<Row>
					<Col>
						<Card>
						  <Card.Header as="h5">Current Status</Card.Header>
						  <Card.Body>
							<Card.Title>Level: {status.level}</Card.Title>
							<Card.Subtitle className="mb-2 text-muted">Cycle: {status.cycle}</Card.Subtitle>
							<Card.Subtitle className="mb-2 text-muted">Hash: {substr(status.hash)}</Card.Subtitle>
							<ProgressBar now={(status.cycleposition / 2048) * 100} />
						  </Card.Body>
						</Card>
					</Col>
					<Col>
						<Card>
					  		<Card.Header as="h5">Recent Activity</Card.Header>
					  		<Card.Body>
					  			<Row>
					  				<Col>
							  			<Card.Title>Baking</Card.Title>
							  			<Card.Subtitle className="mb-2 text-muted">Level: {status.pbl}</Card.Subtitle>
							  			<Card.Subtitle className="mb-2 text-muted">Cycle: {status.pbc}</Card.Subtitle>
					  					<Card.Subtitle className="mb-2 text-muted">Hash: <Card.Link href={"https://tzstats.com/"+status.pbh}>{substr(status.pbh)}</Card.Link></Card.Subtitle>
							  		</Col>
							  		<Col>
							  			<Card.Title>Endorsement</Card.Title>
						  				<Card.Subtitle className="mb-2 text-muted">Level: {status.pel}</Card.Subtitle>
						  				<Card.Subtitle className="mb-2 text-muted">Cycle: {status.pec}</Card.Subtitle>
						  				<Card.Subtitle className="mb-2 text-muted">Hash: <Card.Link href={"https://tzstats.com/"+status.peh}>{substr(status.peh)}</Card.Link></Card.Subtitle>
							  		</Col>
							  	</Row>
							</Card.Body>
						</Card>
					</Col>
				</Row>
				<DelegateInfo delegate={delegate} status={status} />
				<Row>
					<Col>
						<Card>
							<Card.Header as="h5">Next Opportunity</Card.Header>
							<Card.Body>
								<Row>
						  			<Col>
						  				<Card.Title>Baking</Card.Title>
										<Card.Subtitle className="mb-2 text-muted">Level: {status.nbl}</Card.Subtitle>
										<Card.Subtitle className="mb-2 text-muted">Cycle: {status.nbc}</Card.Subtitle>
										<Card.Subtitle className="mb-2 text-muted">Priority: {status.nbp}</Card.Subtitle>
									</Col>
						  			<Col>
						  				<Card.Title>Endorsement</Card.Title>
										<Card.Subtitle className="mb-2 text-muted">Level: {status.nel}</Card.Subtitle>
										<Card.Subtitle className="mb-2 text-muted">Cycle: {status.nec}</Card.Subtitle>
									</Col>
								</Row>
							</Card.Body>
							<Card.Footer>
								<Row>
									<Col><BaconStatus state={connOk} />Last Update: {lastUpdate}</Col>
								</Row>
							</Card.Footer>
						</Card>
					</Col>
				</Row>
			</Container>
		);
	}
}

function substr(g) {
	return String(g).substring(0,10)
}

function BaconStatus(props) {
	return <div className={ "baconstatus baconstatus-" + (props.state ? "green" : "red") }></div>
}

// function AlertDismissible() {
//   const [show, setShow] = useState(true);
// 
//   return (
//     <>
//       <Alert show={show} variant="success">
//         <Alert.Heading>How's it going?!</Alert.Heading>
//         <p>
//           Duis mollis, est non commodo luctus, nisi erat porttitor ligula, eget
//           lacinia odio sem nec elit. Cras mattis consectetur purus sit amet
//           fermentum.
//         </p>
//         <hr />
//         <div className="d-flex justify-content-end">
//           <Button onClick={() => setShow(false)} variant="outline-success">
//             Close me y'all!
//           </Button>
//         </div>
//       </Alert>
// 
//       {!show && <Button onClick={() => setShow(true)}>Show Alert</Button>}
//     </>
//   );
// }

// ========================================

ReactDOM.render(<Bakinbacon />, document.getElementById('bakinbacon'));
