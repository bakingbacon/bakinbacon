import React, { useState, useContext, useEffect, useRef } from 'react';

import Alert from 'react-bootstrap/Alert';
import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Loader from "react-loader-spinner";
import Row from 'react-bootstrap/Row';
import Table from 'react-bootstrap/Table';

import ToasterContext from '../toaster.js';
import { BaconAlert, apiRequest, muToTez, substr } from '../util.js';

import { FaCheckCircle } from 'react-icons/fa';
import { FiMinusCircle } from 'react-icons/fi';
import "react-loader-spinner/dist/loader/css/react-spinner-loader.css";

const DONE        = "done"     // payouts/payouts_types.go
const IN_PROGRESS = "inprog"   // 
const DISABLED    = "disabled" // webserver/api_payouts.go

const Payouts = (props) => {

	const { uiExplorer } = props;

	const [ payoutsMetadata, setPayoutsMetadata ] = useState({});
	const [ payoutsDetail, setPayoutsDetail ]     = useState({});

	const [ isLoading, setIsLoading ]             = useState(true);
	const [ inViewDetail, setViewDetail ]         = useState(false);
	const [ processing, setProcessing ]           = useState(false);
	const [ payoutsDisabled, setPayoutsDisabled ] = useState(false);

	const [ alert, setAlert ] = useState({});
	const addToast            = useContext(ToasterContext);

	const updateViewCycleDetailTimer = useRef();

	const formatter = new Intl.NumberFormat(undefined, {
		minimumFractionDigits: 4,
	})

	useEffect(() => {

		setIsLoading(true);
		getPayoutsMetadata();

		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const resetPayoutsTab = () => {
		setAlert({});
		clearInterval(updateViewCycleDetailTimer.current);
		setIsLoading(true);
		setViewDetail(false);
		getPayoutsMetadata();
	}

	const getPayoutsMetadata = () => {

		const payoutsMetadataApiUrl = window.BASE_URL + "/api/payouts/list"

		apiRequest(payoutsMetadataApiUrl)
		.then((data) => {
			setPayoutsDisabled(data["status"] === DISABLED)
			setPayoutsMetadata(data["metadata"])
		})
		.catch((errMsg) => {
			console.log(errMsg);
			addToast({
				title: "Loading Payouts Error",
				msg: errMsg,
				type: "danger",
			});
		})
		.finally(() => {
			setIsLoading(false);
		});
	}

	const viewCycleDetail = (cycle) => {

		// scroll to top
		window.scrollTo({ top: 0, behavior: 'smooth' });

		const payoutsDetailApiUrl = window.BASE_URL + "/api/payouts/cycledetail?c=" + cycle

		apiRequest(payoutsDetailApiUrl)
		.then((data) => {

			const payoutStatus = data["metadata"]["st"]

			// clear the timer and do a toaster if status is done
			if (payoutStatus === DONE) {
				clearInterval(updateViewCycleDetailTimer.current);
				setAlert({
					msg: "Payouts for cycle "+cycle+" have completed.",
					type: "success",
				});
			}

			// set state for display
			setPayoutsDetail({
				cycle: cycle,
				metadata: data["metadata"],
				rewards: data["payouts"],
			});

			// are we processing payments?
			setProcessing(payoutStatus === IN_PROGRESS)

			// display cycle details
			setViewDetail(true);
		})
		.catch((errMsg) => {
			console.log(errMsg)
			clearInterval(updateViewCycleDetailTimer.current);
			setViewDetail(false);
			addToast({
				title: "Loading Detail Error",
				msg: errMsg,
				type: "danger",
			});
		})
	}

	const sendPayouts = (cycle) => {

		// Clear previous error message
		setAlert({});

		// Show spinner and disable button
		setProcessing(true);
		window.scrollTo({ top: 0, behavior: 'smooth' });

		const sendPayoutsApiUrl = window.BASE_URL + "/api/payouts/sendpayouts"
		const requestOptions = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({"cycle":cycle})
		};

		apiRequest(sendPayoutsApiUrl, requestOptions)
		.then(() => {
			// Only care that it wasn't an error.
			// Set cycle detail to refresh every 30s since we can't process blocks any faster
			updateViewCycleDetailTimer.current = setInterval(() => viewCycleDetail(cycle), 30000);
		})
		.catch((errMsg) => {
			setViewDetail(true);
			clearInterval(updateViewCycleDetailTimer.current);
			setAlert({
				type: "danger",
				msg: errMsg,
			});
		})
	}

	const paidStatusIcon = (amount, opHash) => {
		if (amount === 0) {
			return <FiMinusCircle alt="0 XTZ Reward" title="0 XTZ Reward"/>
		}
		if (opHash !== "") {
			return <a href={"https://"+uiExplorer+"/"+opHash} target={"_blank"} rel={"noreferrer"}><FaCheckCircle /></a>
		}
		return "No"
	}

	if (payoutsDisabled) {
		return (
			<Row><Col><Card>
			<Card.Header as="h5">Payouts</Card.Header>
			<Card.Body>
				<Card.Text>Payouts processing is disabled. Please use an external utility to process your baker rewards.</Card.Text>
			</Card.Body>
			</Card></Col></Row>
		)
	}

	if (isLoading) {
		return (
			<Row>
				<Col className="text-center padded-top-30">
					<Loader type="Circles" color="#EFC700" height={50} width={50} /><br/>Loading Payouts Info...
				</Col>
			</Row>
		)
	}

	if (inViewDetail) {

		const cycle = payoutsDetail["cycle"]
		const bBalance = formatter.format(muToTez(payoutsDetail["metadata"]["b"]))
		const bReward = formatter.format(muToTez(payoutsDetail["metadata"]["br"]))
		const bFee = parseInt(payoutsDetail["metadata"]["f"])
		const cycleStatus = payoutsDetail["metadata"]["st"]
		var totalPayouts = 0;

		return (
			<>
			<Row><Col md={{ span: 2, offset: 10 }}><Button variant="outline-secondary" size="sm" onClick={resetPayoutsTab}>Back to Payouts List</Button></Col></Row>
			<Row>
				<Col>
					<Card>
						<Card.Header as="h5">Payouts Detail for Cycle {cycle}</Card.Header>
						<Card.Body>
							<Card.Text>Below, you can see the detailed rewards information for all of the delegators during this cycle.</Card.Text>
							<Card.Text>In order to make payouts, click the 'Send Payouts' button at the bottom.</Card.Text>
							<Alert variant="warning">
								<b>NOTE:</b> If you are using a Ledger device, understand the following:<br/>
								<ul>
								  <li>Your ledger <b>MUST</b> have the Tezos Wallet app <b>RUNNING</b> during the entire payout process.</li>
								  <li>You will be unable to bake while the wallet app is loaded.</li>
								  <li>You must physically acknowledge each payout transaction by pressing the button on the device.</li>
								  <li>Faiure to acknowledge a payout will time-out the device and may have unforseen consequences.</li>
								</ul>
							</Alert>
							<Card.Text><FiMinusCircle />: Delegator reward is 0.00 XTZ; No payout.</Card.Text>
							<BaconAlert alert={alert} />
							{ processing && <>
							<Card.Text className="text-center" as="div">
							<Loader type="Oval" color="#EFC700" height={35} width={35} style={{"paddingBottom": "10px"}} />
							<p>Processing Payouts.</p>
							<p className="text-muted">This may take up to 30 minutes depending on network speed and number of delegators.<br/>
							'Paid' status will automatically update below as the network accepts the transactions.</p>
							</Card.Text>
							<br/></>
							}
							<Table striped={true}>
								<thead>
									<tr>
										<th>Delegator</th>
										<th>Delegator Balance</th>
										<th>&nbsp;</th>
										<th>Baker Staking Balance</th>
										<th>&nbsp;</th>
										<th>% Share</th>
										<th>&nbsp;</th>
										<th>Baker Reward</th>
										<th>&nbsp;</th>
										<th>Baker Fee</th>
										<th>&nbsp;</th>
										<th>Delegator Reward</th>
										<th>Paid</th>
									</tr>
								</thead>
								<tbody>
									{ Object.keys(payoutsDetail["rewards"]).map((k) => {
										const d = payoutsDetail["rewards"][k];
										var reward = muToTez(d["r"])
										totalPayouts += reward
										const icon = paidStatusIcon(d["r"], d["o"])
										return (
											<tr key={d["d"]}>
												<td>{substr(d["d"])}...</td>
												<td>{formatter.format(muToTez(d["b"]))}&#42793;</td>
												<td>/</td>
												<td>{bBalance}&#42793;</td>
												<td>=</td>
												<td>{d["p"]}%</td>
												<td>*</td>
												<td>{bReward}&#42793;</td>
												<td>-</td>
												<td>{bFee}%</td>
												<td>=</td>
												<td>{formatter.format(reward)}&#42793;</td>
												<td>{ icon }</td>
											</tr>
										)
									})}
									<tr className="row-top-border">
										<td colSpan={11} style={{textAlign: "right"}}>Total Cycle Payouts</td>
										<td colSpan={2} >{formatter.format(totalPayouts)}&#42793;</td>
									</tr>
								</tbody>
							</Table>
							<Card.Text>
							{ cycleStatus === DONE
							?
							"All rewards for this cycle have been processed."
							:
							<Button variant="primary" disabled={processing} onClick={() => sendPayouts(cycle)} type="button" size="sm">Send Payouts</Button>
							}
							</Card.Text>
						</Card.Body>
					</Card>
				</Col>
			</Row>
			</>
		)
	}

	return (
		<>
		<Row>
			<Col>
				<Card>
					<Card.Header as="h5">Payouts</Card.Header>
					<Card.Body>
						<Card.Text>At the end of each cycle, the rewards earned as a baker are unfrozen and deposited to your baker address.</Card.Text>
						<Card.Text>Below, you can see the statistics of each cycle's reward data. To make payouts, click on the 'Details' for a cycle to view the full report and to initiate the payout process.</Card.Text>
						<BaconAlert alert={alert} />
						<Table>
							<thead>
								<tr><th>&nbsp;</th><th>Cycle</th><th>Bakers Balance</th><th>Staking Balance</th><th>Delegated Balance</th><th># Delegators</th><th>Block + Fee Rewards</th><th>&nbsp;</th><th>Total Rewards</th><th>Status</th></tr>
							</thead>
							<tbody>
								{ Object.keys(payoutsMetadata).map((i) => {
									const p = payoutsMetadata[i];
									const totalRewards = p["br"] + p["fr"];
									const status = (p["st"] === DONE ? "Done" : (p["st"] === IN_PROGRESS ? "In Progress" : "Unpaid"))
									return (
										<tr key={p["c"]}>
											<td><Button variant="secondary" onClick={() => viewCycleDetail(p["c"])} type="button" size="sm">Detail</Button></td>
											<td>{p["c"]}</td>
											<td>{formatter.format(muToTez(p["b"]))}&#42793;</td>
											<td>{formatter.format(muToTez(p["sb"]))}&#42793;</td>
											<td>{formatter.format(muToTez(p["db"]))}&#42793;</td>
											<td>{p["nd"]}</td>
											<td>{formatter.format(muToTez(p["br"]))}&#42793;<br/>+&nbsp;{formatter.format(muToTez(p["fr"]))}&#42793;</td>
											<td>=</td>
											<td>{formatter.format(muToTez(totalRewards))}&#42793;</td>
											<td>{status}</td>
										</tr>
									)
								})}
							</tbody>
						</Table>
					</Card.Body>
				</Card>
			</Col>
		</Row>
		</>
	)
};

export default Payouts;
