import React from 'react';

import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Row from 'react-bootstrap/Row';

class NextOpportunities extends React.Component {

	nextBake() {
		const status = this.props.status
		return (
			<>
			<Card.Title>Baking</Card.Title>
			<Card.Subtitle className="mb-2 text-muted">Level: {status.nbl}</Card.Subtitle>
			<Card.Subtitle className="mb-2 text-muted">Cycle: {status.nbc}</Card.Subtitle>
			<Card.Subtitle className="mb-2 text-muted">Priority: {status.nbp}</Card.Subtitle>
			</>
		)
	}
	
	noBaking() {
		return (
			<>
			<Card.Title>Baking</Card.Title>
			<Card.Text>No baking rights found for this cycle.</Card.Text>
			<Card.Text>No baking rights found for next cycle.</Card.Text>
			</>
		)
	}

	nextEndorsement() {
		const status = this.props.status
		return (
			<>
			<Card.Title>Endorsement</Card.Title>
			<Card.Subtitle className="mb-2 text-muted">Level: {status.nel}</Card.Subtitle>
			<Card.Subtitle className="mb-2 text-muted">Cycle: {status.nec}</Card.Subtitle>
			</>
		)
	}
	
	noEndorsements() {
		return (
			<>
			<Card.Title>Endorsement</Card.Title>
			<Card.Text>No endorsements found for this cycle.</Card.Text>
			<Card.Text>No endorsements found for next cycle.</Card.Text>
			</>
		)
	}

	render() {
		const status = this.props.status
		const connOk = this.props.connOk
		const lastUpdate = this.props.lastUpdate

		return (
			<Card>
				<Card.Header as="h5">Next Opportunity</Card.Header>
				<Card.Body>
					<Row>
						<Col>
							{ status.nbl === 0 && this.noBaking() }
							{ status.nbl > 0 && this.nextBake() }
						</Col>
						<Col>
							{ status.nel === 0 && this.noEndorsements() }
							{ status.nel > 0 && this.nextEndorsement() }
						</Col>
					</Row>
				</Card.Body>
			</Card>
		)
	}
}

export default NextOpportunities
		