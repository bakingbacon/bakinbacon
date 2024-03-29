import React, { useState, useContext, useEffect } from 'react';

import Col from 'react-bootstrap/Col';
import Row from 'react-bootstrap/Row';

import Notifications from './notifications.js'
import Rpcservers from './rpcservers.js'
import BakerSettings from './bakersettings.js'

import ToasterContext from '../toaster.js';
import { apiRequest } from '../util.js';

const UI_EXPLORER = "uiexplorer";  // bakinbacon/storage/config.go

const Settings = (props) => {

	const [ settings, updateSettings ] = useState({endpoints:{},notifications:{}})
	const [ isLoading, setIsLoading ] = useState(true);
	const addToast = useContext(ToasterContext);

	useEffect(() => {
		loadSettings();
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const loadSettings = () => {
		const apiUrl = window.BASE_URL + "/api/settings/";
		apiRequest(apiUrl)
		.then((data) => {
			updateSettings((prev) => ({ ...prev, ...data }))
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
	};

	if (isLoading) {
		return (
			<p>Loading...</p>
		)
	}

	return (
		<>
		<Row>
		  <Col md={4}>
			<Rpcservers settings={settings} loadSettings={loadSettings} />
		  </Col>
		  <Col md={8}>
			<BakerSettings settings={settings} loadSettings={loadSettings} />
		  </Col>
		</Row>
		<Row>
		  <Col>
		    <Notifications settings={settings} loadSettings={loadSettings} />
		  </Col>
		</Row>
		</>
	)
}

// Several locations need the UI source
export const GetUiExplorer = (setExplorer) => {
	const apiUrl = window.BASE_URL + "/api/settings/";
	apiRequest(apiUrl)
	.then((data) => {
		setExplorer(data["baker"][UI_EXPLORER]);
	})
	.catch((errMsg) => {
		console.log(errMsg)
		setExplorer("tzstats.com")
	})
}

export default Settings