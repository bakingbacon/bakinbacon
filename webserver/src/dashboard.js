import React, { useEffect, useState } from 'react';

import Card from 'react-bootstrap/Card';
import Col from 'react-bootstrap/Col';
import ProgressBar from 'react-bootstrap/ProgressBar';
import Row from 'react-bootstrap/Row';

import DelegateInfo from './delegateinfo.js'
import NextOpportunities from './nextopportunities.js'
import { BaconAlert, CAN_BAKE, NO_SIGNER, substr } from './util.js'

const BaconDashboard = (props) => {

	const { uiExplorer, delegate, status } = props;
	const [ alert, setAlert ] = useState({})

	useEffect(() => {

		if (status.state === NO_SIGNER) {
			setAlert({
				type: "danger",
				msg: "No signer is configured. If using a ledger, is it plugged in? Unlocked? Baking app open?",
				debug: status.error,
			});
		}

		return null;

		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	return (
		<>
		<Row>
			<Col md={4}>
				<Card>
					<Card.Header as="h5">Current Status</Card.Header>
					<Card.Body>
						<Card.Title>Level: {status.level}</Card.Title>
						<Card.Subtitle className="mb-2 text-muted">Cycle: {status.cycle}</Card.Subtitle>
						<Card.Subtitle className="mb-2 text-muted">Hash: {substr(status.hash)}</Card.Subtitle>
						<ProgressBar now={(status.cycleposition / window.BLOCKS_PER_CYCLE) * 100} />
					</Card.Body>
				</Card>
			</Col>
			<DelegateInfo delegate={delegate} status={status} />
		</Row>

		{ status.state === NO_SIGNER &&
		<BaconAlert alert={alert} />
		}

		{ status.state === CAN_BAKE &&
		<Row>
			<Col md={5}>
				<Card>
					<Card.Header as="h5">Recent Activity</Card.Header>
					<Card.Body>
						<Row>
							<Col>
								<Card.Title>Baking</Card.Title>
								<Card.Subtitle className="mb-2 text-muted">Level: {status.pbl}</Card.Subtitle>
								<Card.Subtitle className="mb-2 text-muted">Cycle: {status.pbc}</Card.Subtitle>
								<Card.Subtitle className="mb-2 text-muted">Hash: <Card.Link href={"https://"+uiExplorer+"/"+status.pbh} target={"_blank"} rel={"noreferrer"}>{substr(status.pbh)}</Card.Link></Card.Subtitle>
							</Col>
							<Col>
								<Card.Title>Endorsement</Card.Title>
								<Card.Subtitle className="mb-2 text-muted">Level: {status.pel}</Card.Subtitle>
								<Card.Subtitle className="mb-2 text-muted">Cycle: {status.pec}</Card.Subtitle>
								<Card.Subtitle className="mb-2 text-muted">Hash: <Card.Link href={"https://"+uiExplorer+"/"+status.peh} target={"_blank"} rel={"noreferrer"}>{substr(status.peh)}</Card.Link></Card.Subtitle>
							</Col>
						</Row>
					</Card.Body>
				</Card>
			</Col>
			<Col md={7}>
				<NextOpportunities status={status} />
			</Col>
		</Row>
		}
		</>
	)
}

export default BaconDashboard
