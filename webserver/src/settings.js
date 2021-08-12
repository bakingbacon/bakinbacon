import React, { useState, useContext, useEffect } from 'react';

import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form'
import ListGroup from 'react-bootstrap/ListGroup';
import Row from 'react-bootstrap/Row';

import ToasterContext from './toaster.js';
import { BASE_URL, CHAINID_GRANADANET, apiRequest } from './util.js';


const Settings = (props) => {

	const [newRpc, setNewRpc] = useState("");
	const [rpcEndpoints, setRpcEndpoints] = useState({});
	const [telegramConfig, setTelegramConfig] = useState({apikey:"", chatids:""})
	const [emailConfig, setEmailConfig] = useState({});
	const [isLoading, setIsLoading] = useState(true);
	const addToast = useContext(ToasterContext);

	useEffect(() => {
		loadSettings();
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const handleNewRpcChange = (event) => {
		setNewRpc(event.target.value);
	}

	const loadSettings = () => {
		const apiUrl = BASE_URL + "/api/settings/";
		apiRequest(apiUrl)
		.then((data) => {

			const tConfig = data.notifications.telegram;
			if (tConfig.chatids == null) {
				tConfig.chatids = []
			}
			tConfig.chatids = tConfig.chatids.join() // explode array into a string
			if (Object.keys(tConfig).length !== 0) {
				setTelegramConfig(tConfig)
			}

			setRpcEndpoints(data.endpoints || {});
			setEmailConfig(data.notifications.email || {})
		})
		.catch((errMsg) => {
			console.log(errMsg);
			addToast({
				title: "Loading Settings Error",
				msg: errMsg,
				type: "danger",
			});
		})
		.finally(() => {
			setIsLoading(false);
		})
	}

	const handleTelegramChange = (e) => {
		const { name, value } = e.target;
		setTelegramConfig((prev) => ({
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

	const addRpc = () => {

		// Cheezy sanity check
		const rpcToAdd = stripSlash(newRpc);
		if (rpcToAdd.length < 10) {
			addToast({
				title: "Add RPC Error",
				msg: "That does not appear a valid URL",
				type: "warning",
				autohide: 3000,
			});
			return;
		}

		console.log("Adding RPC endpoint: " + rpcToAdd)

		// Sanity check the endpoint first by fetching the current head and checking the protocol.
		// This has the added effect of forcing upgrades for new protocols.
		apiRequest(rpcToAdd + "/chains/main/blocks/head/header")
		.then((data) => {
			const chainId = data.chain_id;
			if (chainId !== CHAINID_GRANADANET) {
				throw new Error("RPC chain ("+chainId+") does not match "+CHAINID_GRANADANET+". Please use a correct RPC server.");
			}

			// RPC is good! Add it via API.
			const apiUrl = BASE_URL + "/api/settings/addendpoint"
			const postData = {rpc: rpcToAdd}
			handlePostAPI(apiUrl, postData).then(() => {
				addToast({
					title: "RPC Success",
					msg: "Added RPC Server",
					type: "success",
					autohide: 3000,
				});
			});
		})
		.catch((errMsg) => {
			console.log(errMsg);
			addToast({
				title: "Add RPC Error",
				msg: "There was an error in validating the RPC URL: " + errMsg,
				type: "danger",
			});
		})
		.finally(() => {
			setNewRpc("");
		});
	}

	const delRpc = (rpc) => {
		console.log("Deleting RPC endpoint: " + rpc)
		const apiUrl = BASE_URL + "/api/settings/deleteendpoint"
		handlePostAPI(apiUrl, Number(rpc)).then(() => {
			addToast({
				title: "RPC Success",
				msg: "Deleted RPC Server",
				type: "success",
				autohide: 3000,
			});
		})
		.finally(() => {
			setNewRpc("");
		});
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

	if (isLoading) {
		return (
			<p>Loading...</p>
		)
	}

	return (
		<>
		<Row>
		  <Col md={5}>
			<Card>
			  <Card.Header as="h5">RPC Servers</Card.Header>
			  <Card.Body>
			  <Card.Text>BakinBacon supports multiple RPC servers for increased redundancy against network issues and will always use the most up-to-date server.</Card.Text>
			  </Card.Body>
			  <ListGroup variant="flush">
				{ Object.keys(rpcEndpoints).map((rpcId) => {
					return <ListGroup.Item key={rpcId}><Button onClick={() => delRpc(rpcId)} variant="danger" size="sm" type="button">{'X'}</Button> {rpcEndpoints[rpcId]}</ListGroup.Item>
				})}
			  </ListGroup>
			  <Card.Body>
				<Form.Row>
				  <Form.Group as={Col} md="9">
					<Form.Control type="text" placeholder="https://" value={newRpc} onChange={handleNewRpcChange} />
					<Form.Text className="text-muted">Add RPC Server</Form.Text>
				  </Form.Group>
				  <Form.Group as={Col} md="3">
					<Button variant="primary" onClick={addRpc} type="button" size="sm">Submit</Button>
				  </Form.Group>
				</Form.Row>
			  </Card.Body>
			</Card>
		  </Col>
		</Row>
		<Row>
		  <Col>
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
                            <Button variant="primary" onClick={saveTelegram} type="button" size="sm">Save</Button>
                          </Form.Group>
                        </Form.Row>
		              </Card.Body>
		            </Card>
		          </Col>
		          <Col md="6">
		            <Card>
		              <Card.Header as="h5">Email</Card.Header>
		            </Card>
		          </Col>
		        </Row>
		      </Card.Body>
		    </Card>
		  </Col>
		</Row>
		</>
	)
}

function stripSlash(d) {
	return d.endsWith('/') ? d.substr(0, d.length - 1) : d;
}

export default Settings
