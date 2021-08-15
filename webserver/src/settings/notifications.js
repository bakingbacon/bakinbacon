import React, { useState, useContext, useEffect } from 'react';

import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form'
import Row from 'react-bootstrap/Row';

import ToasterContext from '../toaster.js';
import { BASE_URL, apiRequest } from '../util.js';


const Notifications = (props) => {

	const { settings, loadSettings } = props;

	const [telegramConfig, setTelegramConfig] = useState({apikey:"", chatids:"", enabled:false})
	const [emailConfig, setEmailConfig] = useState({smtphost:""});
	const addToast = useContext(ToasterContext);

	useEffect(() => {
		const config = settings.notifications;
		const tConfig = config.telegram;
		if (tConfig.chatids == null) {
			tConfig.chatids = []
		}

		if (Object.keys(tConfig).length !== 0) {
			setTelegramConfig(tConfig)
		}

		setEmailConfig(config.email)

	}, [settings]);

	const handleTelegramChange = (e) => {
		let { name, value } = e.target;
		if (name === "tenabled") {
			console.log(e.target.checked)
			name = "enabled"
			value = (e.target.checked ? false : true);
		}
		setTelegramConfig((prev) => ({
			...prev,
			[name]: value
		}));
	}

	const handleEmailChange = (e) => {
		const { name, value } = e.target;
		setEmailConfig((prev) => ({
			...prev,
			[name]: value
		}));
	}

	const saveTelegram = (e) => {

		// Validation first
		const chatIds = telegramConfig.chatids.split(/[ ,]/);
		for (let i = 0; i < chatIds.length; i++) {
			chatIds[i] = Number(chatIds[i])  // Convert strings to int
			if (isNaN(chatIds[i])) {
				addToast({
					title: "Invalid ChatId",
					msg: "Telegram chatId must be a positive or negative number.",
					type: "danger",
					autohide: 6000,
				});
				return;
			}
		}

		const botapikey = telegramConfig.apikey;
		const regex = new RegExp(/\d{9}:[0-9A-Za-z_-]{35}/);
		if (!regex.test(botapikey)) {
			addToast({
				title: "Invalid Bot API Key",
				msg: "Provided API key does not match known pattern.",
				type: "danger",
				autohide: 6000,
			});
			return;
		}

		// Validations complete
		const apiUrl = BASE_URL + "/api/settings/savetelegram"
		const postData = {
			chatids: chatIds,
			apikey: botapikey,
		};
		handlePostAPI(apiUrl, postData).then(() => {
			addToast({
				title: "Save Telegram Success",
				msg: "Saved Telegram config. You should receive a test message soon. If not, check your config values and save again.",
				type: "success",
				autohide: 3000,
			});
		})
	}

	// Add/Delete RPC, and Save Telegram/Email RPCs use POST and only care if failure.
	// On 200 OK, refresh settings
	const handlePostAPI = (url, data) => {

		const requestOptions = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(data)
		};

		return apiRequest(url, requestOptions)
			.then(() => {
				loadSettings();
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
          <Card.Header as="h5">Notifications</Card.Header>
          <Card.Body>
            <Card.Text>Bakin'Bacon can send notifications on certain actions: Not enough bond, cannot find ledger, etc. Fill in the required information below to enable different notification destinations. A test message will be sent on 'Save'.</Card.Text>
            <Row>
              <Col md="6">
                <Card>
                  <Card.Header as="h5">Telegram</Card.Header>
                  <Card.Body>
                    <Form.Row>
                      <Form.Group as={Col}>
                        <Form.Text as="span">Chat Ids</Form.Text>
                        <Form.Control type="text" name="chatids" value={telegramConfig.chatids} onChange={handleTelegramChange} />
                        <Form.Text className="text-muted">Separate multiple chatIds with ','</Form.Text>
                      </Form.Group>
                    </Form.Row>
                    <Form.Row>
                      <Form.Group as={Col}>
                        <Form.Text as="span">Bot API Key</Form.Text>
                        <Form.Control type="text" name="apikey" value={telegramConfig.apikey} onChange={handleTelegramChange} />
                      </Form.Group>
                    </Form.Row>
                    <Form.Row>
                      <Form.Group as={Col}>
                        <Form.Check type="checkbox" name="tenabled" checked={telegramConfig.enabled} onChange={handleTelegramChange} label="Enabled" />
                      </Form.Group>
                    </Form.Row>
                    <Form.Row>
                      <Form.Group as={Col}>
                        <Button variant="primary" onClick={saveTelegram} type="button" size="sm">Save</Button>
                      </Form.Group>
                    </Form.Row>
                  </Card.Body>
                </Card>
              </Col>
              <Col md="6">
                <Card>
                  <Card.Header as="h5">Email</Card.Header>
                  <Card.Body>
                    <Form.Row>
                      <Form.Group as={Col}>
                        <Form.Text as="span">SMTP Server</Form.Text>
                        <Form.Control type="text" name="smtphost" value={emailConfig.smtphost} onChange={handleEmailChange} />
                      </Form.Group>
                    </Form.Row>
                  </Card.Body>
                </Card>
              </Col>
            </Row>
          </Card.Body>
        </Card>
        </>
	)
}

export default Notifications
