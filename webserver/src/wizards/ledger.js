import React, { useState } from 'react';

import Button from 'react-bootstrap/Button';
import Card from 'react-bootstrap/Card';

//const BASE_URL = ""
//const BASE_URL = "http://10.10.10.203:8082"

const WizardLedger = (props) => {

	const { onFinishWizard } = props;

	const [ step, setStep ] = useState(1)
	
	// This renders inside parent <Card.Body>
	if (step === 1) {
		return(
			<Card.Text>
			<p>This is ledger wizard step 1</p>
			</Card.Text>
		);
	}
	
	// Default shows error
	return (
		<Button variant="warning" block onClick={onFinishWizard}>Uh oh... something went wrong.</Button>
	);
}

export default WizardLedger
