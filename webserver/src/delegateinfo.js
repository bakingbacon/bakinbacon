import React, { useEffect, useState, useContext } from 'react';

import NumberFormat from 'react-number-format';

import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import Loader from "react-loader-spinner";

import ToasterContext from './toaster.js';
import { NO_SIGNER, NOT_REGISTERED, CAN_BAKE, apiRequest } from './util.js';

import "react-loader-spinner/dist/loader/css/react-spinner-loader.css";


const DelegateInfo = (props) => {

	const { delegate, status } = props;

	let nbDelegators = 0;
	const [ balanceInfo, setBalanceInfo ] = useState({
		frozen: 0,
		spendable: 0,
		total: 0,
		stakingBalance: 0,
		delegatedBalance: 0,
		nbDelegators: 0,
	});
	const [ isLoading, setIsLoading ] = useState(false);
	const [ connOk, setConnOk ] = useState(true);
	const addToast = useContext(ToasterContext);

	useEffect(() => {

		if (status.state === NOT_REGISTERED || status.state === NO_SIGNER) {
			return null;
		}

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

		// Fetch delegator info which is only necessary when looking at the UI
		const apiUrl = "http://granadanet-us.rpc.bakinbacon.io/chains/main/blocks/head/context/delegates/" + delegate
		apiRequest(apiUrl)
		.then(data => {

			const balance = parseInt(data.balance, 10);
			const frozenBalance = parseInt(data.frozen_balance, 10);
			const spendable = balance - frozenBalance;

			// If we loose a delegator, show message
			const newNbDelegators = (data.delegated_contracts.length - 1);  // Don't count ourselves

			if (nbDelegators > 0 && newNbDelegators > nbDelegators) {
				// Gained
				addToast({
					title: "New Delegator!",
					msg: "You gained a delegator!",
					type: "primary",
				});
			} else if (nbDelegators > 0 && newNbDelegators < nbDelegators) {
				// Lost
				addToast({
					title: "Lost Delegator",
					msg: "You lost a delegator! No big deal; it happens to everyone.",
					type: "info",
				});
			}

			// Update variable
			nbDelegators = newNbDelegators;

			// Update object for redraw
			setBalanceInfo({
				total: (balance / 1e6).toFixed(2),
				frozen: (frozenBalance / 1e6).toFixed(2),
				spendable: (spendable / 1e6).toFixed(2),
				stakingBalance: (parseInt(data.staking_balance, 10) / 1e6).toFixed(2),
				delegatedBalance: (parseInt(data.delegated_balance, 10) / 1e6).toFixed(2),
				nbDelegators: newNbDelegators,
			});

		})
		.catch((errMsg) => {
			console.log(errMsg)
			setConnOk(false);
			addToast({
				title: "Loading Delegate Error",
				msg: errMsg,
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

	if (status.state === NOT_REGISTERED || status.state === NO_SIGNER) {
		return null;
	}

	// Fallback
	return (<Col>Delegate info currently unavailable.</Col>);
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