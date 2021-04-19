import React from 'react';

import NumberFormat from 'react-number-format';

import Alert from 'react-bootstrap/Alert'
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import Loader from "react-loader-spinner";
import Row from 'react-bootstrap/Row';

import DelegateRegister from './delegateregister.js'

import "react-loader-spinner/dist/loader/css/react-spinner-loader.css";

// baconclient/baconstatus.go
const CAN_BAKE = "canbake"
const LOW_BALANCE = "lowbal"
const NOT_REGISTERED = "noreg"
const NO_SIGNER = "nosign"

class DelegateInfo extends React.Component {

	constructor(props) {
		super(props);

		this.state = {
			status: props.status,
			frozen: 0,
			spendable: 0,
			total: 0,
			stakingBalance: 0,
			delegatedBalance: 0,
			nbDelegators: 0,
			isLoading: false,
			connOk: true,
		}
	};
	
	componentDidMount() {
		this.setState({ isLoading: true })
		this.fetchDelegateInfo();
		// Update every 5 minutes
		this.fetchDelegateInfoTimer = setInterval(() => this.fetchDelegateInfo(), 1000*60*5);
	}
	
	componentWillUnmount() {
		clearInterval(this.fetchDelegateInfoTimer);
		this.fetchDelegateInfoTimer = null;
	}
	
	fetchDelegateInfo() {

		const dState = this.state.status.state

		// If baker is not yet revealed/registered, we just need to monitor basic balance so we can display the button
		if (dState === NOT_REGISTERED) {

			const balanceUrl = "https://florence-tezos.giganode.io/chains/main/blocks/head/context/contracts/" + this.props.delegate
			fetch(balanceUrl)
				.then(response => {
					if (!response.ok) {
						throw new Error("Error fetching balance");
					}
					return response;
				})
				.then(response => response.json())
				.then(data => {
					this.setState({
						spendable: (parseInt(data.balance, 10) / 1e6).toFixed(1),
						isLoading: false,
					})
				})
				.catch(error => {
					this.setState({
						connOk: false,
						isLoading: false,
					});
					console.log(error)
					// TODO: Toaster
				})
			
			return
		}

		// Fetch delegator info which is only necessary when looking at the UI
		const apiUrl = "https://florence-tezos.giganode.io/chains/main/blocks/head/context/delegates/" + this.props.delegate
		fetch(apiUrl)
			.then(response => {
				if (!response.ok) {
					throw new Error("Error fetching delegate info");
				}
				response.json().then(data => {
					const balance = parseInt(data.balance, 10);
					const frozenBalance = parseInt(data.frozen_balance, 10);
					const spendable = balance - frozenBalance;
					const nbDels = data.delegated_contracts.length;
					const stakingBalance = parseInt(data.staking_balance, 10);
					const delegatedBalance = parseInt(data.delegated_balance, 10);
					this.setState({
						frozen: (frozenBalance / 1e6).toFixed(2),
						spendable: (spendable / 1e6).toFixed(2),
						total: (balance / 1e6).toFixed(2),
						stakingBalance: (stakingBalance / 1e6).toFixed(2),
						delegatedBalance: (delegatedBalance / 1e6).toFixed(2),
						nbDelegators: nbDels,
						isLoading: false,
					});
				});
			})
			.catch(error => {
				this.setState({
					connOk: false,
					isLoading: false
				});
				// TODO: Toaster
				console.log(error);
			});
	}
	
	render() {
		const isLoading = this.state.isLoading
		const isConnOk = this.state.connOk
		const hasError = this.state.status.err
		const bakeState = this.state.status.state
		
		if (isLoading || !isConnOk) {
			return (
				<>
				<Row>
					<Col className="text-center"><Loader type="Circles" color="#EFC700" height={50} width={50} /><br/>Loading Baker Info...</Col>
				</Row>
				{ hasError &&
					<Row><Col className="text-center"><Alert variant="danger">{hasError}</Alert></Col></Row>
				}
				</>
			)
		}
		
		if (bakeState === CAN_BAKE) {
			return (
				<>
				<Row>
					<Col md={4}><DelegateBalances frozen={this.state.frozen} spendable={this.state.spendable} total={this.state.total} /></Col>
					<Col md={4}><DelegateStats nbDels={this.state.nbDelegators} stakeBal={this.state.stakingBalance} deleBal={this.state.delegatedBalance} /></Col>
				</Row>
				</>
			)
		}
		
		if (bakeState === NOT_REGISTERED) {
			return (<DelegateRegister pkh={this.props.delegate} spendable={this.state.spendable} />)
		}
	}
}

class DelegateBalances extends React.Component {

	render() {
		return (
			<Card>
				<Card.Header as="h5">Delegate Balances</Card.Header>
				<ListGroup variant="flush">
					<ListGroup.Item><div className="stats-title">Frozen:</div> <NumberFormat value={this.props.frozen} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
					<ListGroup.Item><div className="stats-title">Spendable:</div> <NumberFormat value={this.props.spendable} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
					<ListGroup.Item><div className="stats-title">Total:</div> <NumberFormat value={this.props.total} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
				</ListGroup>
			</Card>
		)
	}
}

class DelegateStats extends React.Component {

	render() {
		return (
			<Card>
				<Card.Header as="h5">Delegated Stats</Card.Header>
				<ListGroup variant="flush">
					<ListGroup.Item><div className="stats-title-w">Delegated Balance:</div> <NumberFormat value={this.props.deleBal} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
					<ListGroup.Item><div className="stats-title-w">Staking Balance:</div> <NumberFormat value={this.props.stakeBal} displayType={'text'} suffix={'ꜩ'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
					<ListGroup.Item><div className="stats-title-w"># Delegators:</div> <NumberFormat value={this.props.nbDels} displayType={'text'} renderText={value => <div className="stats-val">{value}</div>} /></ListGroup.Item>
				</ListGroup>
			</Card>
		)
	}
}

export default DelegateInfo