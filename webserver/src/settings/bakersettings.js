import React, { useState, useContext, useEffect } from 'react';

import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form'

import ToasterContext from '../toaster.js';
import { apiRequest } from '../util.js';


const BakerSettings = (props) => {

	const { settings, loadSettings } = props;

	const [ bakerSettings, setBakerSettings] = useState(settings["baker"]);
	const addToast = useContext(ToasterContext);

	useEffect(() => {
		setBakerSettings(settings["baker"]);
	}, [settings]);

	const handleUpdate = (event) => {
		setBakerSettings((prev) => ({
			...prev,
			[event.target.name]: event.target.value
		}));
	}

	const validateBakerFee = () => {
		const fee = Number(bakerSettings["bakerfee"]);
		if (isNaN(fee)) {
			return false
		}
		if (fee < 1 || fee > 99) {
			return false
		}
		return true;
	}

	const updateBakerSettings = () => {

		// Validation
		if (!validateBakerFee()) {
			addToast({
				title: "Settings Error",
				msg: "Baker fee must be an integer value between 1 and 99",
				type: "danger",
				autohide: 3000,
			});
			return
		}

		// Validations passed; submit changes
		const bakerSettingsApiUrl = window.BASE_URL + "/api/settings/bakersettings"
		const postData = bakerSettings;

		const requestOptions = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(postData)
		};

		apiRequest(bakerSettingsApiUrl, requestOptions)
		.then(() => {
			loadSettings();
			addToast({
				title: "Saved Settings",
				msg: "Successfully saved baker settings.",
				type: "info",
				autohide: 3000,
			});
		})
		.catch((errMsg) => {
			console.log(errMsg);
			addToast({
				title: "Settings Error",
				msg: errMsg,
				type: "danger",
			});
		});
	}

	return (
		<>
		<Card>
			<Card.Header as="h5">Baker Settings</Card.Header>
			<Card.Body>
				<Card.Text>The following parameters handle different aspects of running a bakery.</Card.Text>
				<Form.Row>
					<Form.Group as={Col} md="9">
						<Form.Control type="text" name="bakerfee" value={bakerSettings["bakerfee"]} onChange={(e) => handleUpdate(e)} />
						<Form.Text className="text-muted">Baker Fee - This is how much your bakery charges delegators. Please use whole numbers without the % sign.</Form.Text>
					</Form.Group>
				</Form.Row>
				<Form.Row>
					<Form.Group as={Col} md="4">
						<Button variant="primary" onClick={updateBakerSettings} type="button" size="sm">Save Settings</Button>
					</Form.Group>
				</Form.Row>
			</Card.Body>
		</Card>
		</>
	)
}

export default BakerSettings
