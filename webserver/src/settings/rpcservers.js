import React, { useState, useContext, useEffect } from 'react';

import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form'
import ListGroup from 'react-bootstrap/ListGroup';

import ToasterContext from '../toaster.js';
import { CHAINIDS, apiRequest } from '../util.js';


const Rpcservers = (props) => {

	const { settings, loadSettings } = props;

	const [newRpc, setNewRpc] = useState("");
	const [rpcEndpoints, setRpcEndpoints] = useState({});
	const addToast = useContext(ToasterContext);

	useEffect(() => {
		setRpcEndpoints(settings.endpoints);
	}, [settings]);

	const handleNewRpcChange = (event) => {
		setNewRpc(event.target.value);
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
			const rpcChainId = data.chain_id;
			const networkChainId = CHAINIDS[window.NETWORK]
			if (rpcChainId !== networkChainId) {
				throw new Error("RPC chain ("+rpcChainId+") does not match "+networkChainId+". Please use a correct RPC server.");
			}

			// RPC is good! Add it via API.
			const apiUrl = window.BASE_URL + "/api/settings/addendpoint"
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
		const apiUrl = window.BASE_URL + "/api/settings/deleteendpoint"
		const postData = {rpc: Number(rpc)}
		handlePostAPI(apiUrl, postData).then(() => {
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

	return (
		<>
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
		</>
	)
}

function stripSlash(d) {
	return d.endsWith('/') ? d.substr(0, d.length - 1) : d;
}

export default Rpcservers
