import React, { useState, useContext, useEffect, useRef } from 'react';

import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import Loader from "react-loader-spinner";
import Row from 'react-bootstrap/Row';
import Table from 'react-bootstrap/Table';

import ToasterContext from '../toaster.js';
import { BaconAlert, apiRequest, muToTez, substr } from '../util.js';

import { FaCheckCircle } from 'react-icons/fa';
import "react-loader-spinner/dist/loader/css/react-spinner-loader.css";

const Payouts = () => {

	const [ payoutsMetadata, setPayoutsMetadata ] = useState({});
	const [ payoutsDetail, setPayoutsDetail ]     = useState({});
	const [ payoutsStatus, setPayoutsStatus ]     = useState({});

	const [ isLoading, setIsLoading ]                 = useState(true);
	const [ inViewDetail, setViewDetail ]             = useState(false);

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

	const getPayoutsMetadata = () => {

		const payoutsMetadataApiUrl = window.BASE_URL + "/api/payouts/list"

		apiRequest(payoutsMetadataApiUrl)
		.then((data) => {
			setPayoutsMetadata(data)
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

		const payoutsDetailApiUrl = window.BASE_URL + "/api/payouts/cycledetail?c=" + cycle

		apiRequest(payoutsDetailApiUrl)
		.then((data) => {
			if (data["done"]) {
				clearInterval(updateViewCycleDetailTimer.current);
			}
			setPayoutsDetail({
				cycle: cycle,
				metadata: payoutsMetadata[cycle],
				rewards: data["detail"],
			});
			setViewDetail(true);
		})
		.catch((errMsg) => {
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

		const sendPayoutsApiUrl = window.BASE_URL + "/api/payouts/send?c=" + cycle

		apiRequest(sendPayoutsApiUrl)
		.then(() => {
			// Only care that it wasn't an error. Set cycle detail to refresh every 2s
			updateViewCycleDetailTimer.current = setInterval(() => viewCycleDetail(cycle), 2000);
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
		var totalPayouts = 0;

		return (
			<>
			<Row>
				<Col>
					<Card>
						<Card.Header as="h5">Payouts Detail for Cycle {cycle}</Card.Header>
						<Card.Body>
							<Card.Text>Below, you can see the detailed rewards information for all of the delegators during this cycle.</Card.Text>
							<Card.Text>In order to make payouts, click the 'Send Payouts' button at the bottom. If you are using a Ledger device, please note: 1) your ledger <b>MUST</b> have the Tezos Wallet app <b>running</b>, and 2) you will be unable to bake while the wallet app is loaded.</Card.Text>
							<BaconAlert alert={alert} />
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
										<th>Delegator Reward</th>
										<th>Paid</th>
									</tr>
								</thead>
								<tbody>
									{ payoutsDetail.rewards.map((d) => {
										var reward = muToTez(d["r"])
										totalPayouts += reward
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
												<td>=</td>
												<td>{formatter.format(reward)}&#42793;</td>
												<td>{ d["o"] 
												? <a href={"https://tzstats.com/"+d["o"]} target={"_blank"} rel={"noreferrer"}><FaCheckCircle /></a>
												: "No"
												}
												</td>
											</tr>
										)
									})}
									<tr className="row-top-border">
										<td colSpan={9} Style="text-align:right;">Total Cycle Payouts</td>
										<td colSpan={2}>{formatter.format(totalPayouts)}&#42793;</td>
									</tr>
								</tbody>
							</Table>
							<Card.Text><Button variant="primary" onClick={() => sendPayouts(cycle)} type="button" size="sm">Send Payouts</Button></Card.Text>
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
								<tr><th>&nbsp;</th><th>Cycle</th><th>Bakers Balance</th><th>Staking Balance</th><th>Delegated Balance</th><th># Delegators</th><th>Block Rewards</th><th>Txn Fee Rewards</th></tr>
							</thead>
							<tbody>
								{ payoutsMetadata.map((p, i) => {
									return (
										<tr key={p["c"]}>
											<td><Button variant="secondary" onClick={() => viewCycleDetail(p["c"])} type="button" size="sm">Detail</Button></td>
											<td>{p["c"]}</td>
											<td>{formatter.format(muToTez(p["b"]))}&#42793;</td>
											<td>{formatter.format(muToTez(p["sb"]))}&#42793;</td>
											<td>{formatter.format(muToTez(p["db"]))}&#42793;</td>
											<td>{p["nd"]}</td>
											<td>{formatter.format(muToTez(p["br"]))}&#42793;</td>
											<td>{formatter.format(muToTez(p["fr"]))}&#42793;</td>
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
