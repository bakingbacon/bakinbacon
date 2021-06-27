import React from 'react';

import Card from 'react-bootstrap/Card';
import Col from 'react-bootstrap/Col';
import ProgressBar from 'react-bootstrap/ProgressBar';
import Row from 'react-bootstrap/Row';

import DelegateInfo from './delegateinfo.js'
import NextOpportunities from './nextopportunities.js'

const BakinDashboard = (props) => {

	const { status, delegate } = props;

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
						<ProgressBar now={(status.cycleposition / 2048) * 100} />
					</Card.Body>
				</Card>
			</Col>
			<DelegateInfo delegate={delegate} status={status} />
		</Row>
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
			<Col md={7}>
				<NextOpportunities status={status} />
			</Col>
		</Row>
		</>
	)
}

function substr(g) {
	return String(g).substring(0,10)
}

export default BakinDashboard