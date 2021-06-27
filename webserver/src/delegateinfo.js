import React, { useEffect, useState, useContext } from 'react';

import NumberFormat from 'react-number-format';

import Alert from 'react-bootstrap/Alert'
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import Loader from "react-loader-spinner";

import DelegateRegister from './delegateregister.js'
import ToasterContext from './toaster.js';

import "react-loader-spinner/dist/loader/css/react-spinner-loader.css";

// baconclient/baconstatus.go
const CAN_BAKE = "canbake"
//const LOW_BALANCE = "lowbal"
const NOT_REGISTERED = "noreg"
//const NO_SIGNER = "nosign"

const DelegateInfo = (props) => {

	const delegate = props.delegate;

	const [ balanceInfo, setBalanceInfo ] = useState({
		frozen: 0,
		spendable: 0,
		total: 0,
		stakingBalance: 0,
		delegatedBalance: 0,
		nbDelegators: 0,
	});

	const status = props.status
	const [ isLoading, setIsLoading ] = useState(false);
	const [ connOk, setConnOk ] = useState(true);
	const addToast = useContext(ToasterContext);

	useEffect(() => {
		setIsLoading(true);
		fetchDelegateInfo();

		// Update every 5 minutes
		let fetchDelegateInfoTimer = setInterval(() => fetchDelegateInfo(), 1000 * 60 * 5);
		return () => {
			// componentWillUnmount()
			clearInterval(fetchDelegateInfoTimer);
			fetchDelegateInfoTimer = null;
		};
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const fetchDelegateInfo = () => {

		const dState = status.state

		// If baker is not yet revealed/registered, we just need to monitor basic balance so we can display the button
		if (dState === NOT_REGISTERED) {

			const balanceUrl = "http://florencenet-us.rpc.bakinbacon.io/chains/main/blocks/head/context/contracts/" + delegate
			fetch(balanceUrl)
			.then(response => {
				if (!response.ok) {
					throw new Error("Error fetching balance");
				}
				return response.json();
			}).then(data => {
				setBalanceInfo((balanceInfo) => ({
					...balanceInfo, spendable: (parseInt(data.balance, 10) / 1e6).toFixed(1)
				}));
			})
			.catch(e => {
				console.log(e)
				setConnOk(false);
				addToast({
					title: "Loading Delegate Error",
					msg: e.message,
					type: "danger",
				});
			})
			.finally(() => {
				setIsLoading(false);
			})
			
			return;
		}

		// Fetch delegator info which is only necessary when looking at the UI
		const apiUrl = "http://florencenet-us.rpc.bakinbacon.io/chains/main/blocks/head/context/delegates/" + delegate
		fetch(apiUrl)
		.then(response => {
			if (!response.ok) {
				throw new Error("Error fetching delegate info");
			}
			return response.json();
		}).then(data => {

			const balance = parseInt(data.balance, 10);
			const frozenBalance = parseInt(data.frozen_balance, 10);
			const spendable = balance - frozenBalance;

			setBalanceInfo({
				total: (balance / 1e6).toFixed(2),
				frozen: (frozenBalance / 1e6).toFixed(2),
				spendable: (spendable / 1e6).toFixed(2),
				stakingBalance: (parseInt(data.staking_balance, 10) / 1e6).toFixed(2),
				delegatedBalance: (parseInt(data.delegated_balance, 10) / 1e6).toFixed(2),
				nbDelegators: data.delegated_contracts.length,
			});
		})
		.catch(e => {
			console.log(e)
			setConnOk(false);
			addToast({
				title: "Loading Delegate Error",
				msg: e.message,
				type: "danger",
			});
		})
		.finally(() => {
			setIsLoading(false);
		});
	}

	// Returns	
	if (isLoading || !connOk) {
		return (
			<>
			<Col md={8} className="text-center padded-top-30">
				<Loader type="Circles" color="#EFC700" height={50} width={50} /><br/>Loading Baker Info...
			</Col>
			{ status.err &&
				<Col md={8} className="text-center"><Alert variant="danger">{status.err}</Alert></Col>
			}
			</>
		)
	}
		
	if (status.state === CAN_BAKE) {
		return (
			<>
			<Col md={4}><DelegateBalances frozen={balanceInfo.frozen} spendable={balanceInfo.spendable} total={balanceInfo.total} /></Col>
			<Col md={4}><DelegateStats nbDels={balanceInfo.nbDelegators} stakeBal={balanceInfo.stakingBalance} deleBal={balanceInfo.delegatedBalance} /></Col>
			</>
		)
	}
	
	if (status.state === NOT_REGISTERED) {
		return (<DelegateRegister pkh={delegate} spendable={balanceInfo.spendable} />)
	}
	
	// Fallback
	return (<Col>Unable to fetch delegate info.</Col>);
}

const DelegateBalances = (props) => {

	return (
		<Card>
			<Card.Header as="h5">Baker Balances</Card.Header>
			<ListGroup variant="flush">
				<ListGroup.Item><div className="stats-title">Frozen:</div> <NumberFormat value={props.frozen} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
				<ListGroup.Item><div className="stats-title">Spendable:</div> <NumberFormat value={props.spendable} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
				<ListGroup.Item><div className="stats-title">Total:</div> <NumberFormat value={props.total} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
			</ListGroup>
		</Card>
	)
}

const DelegateStats = (props) => {

	return (
		<Card>
			<Card.Header as="h5">Baker Stats</Card.Header>
			<ListGroup variant="flush">
				<ListGroup.Item><div className="stats-title-w">Delegated Balance:</div> <NumberFormat value={props.deleBal} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
				<ListGroup.Item><div className="stats-title-w">Staking Balance:</div> <NumberFormat value={props.stakeBal} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
				<ListGroup.Item><div className="stats-title-w"># Delegators:</div> <NumberFormat value={props.nbDels} displayType={'text'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
			</ListGroup>
		</Card>
	)
}

export default DelegateInfo