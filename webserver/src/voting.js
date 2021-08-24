import React, { useState, useContext, useEffect } from 'react';

import Alert from 'react-bootstrap/Alert'
import Button from 'react-bootstrap/Button';
import Col from 'react-bootstrap/Col';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import Loader from "react-loader-spinner";
import Row from 'react-bootstrap/Row';

import ToasterContext from './toaster.js';
import { BaconAlert, apiRequest } from './util.js';

import "react-loader-spinner/dist/loader/css/react-spinner-loader.css";

const Voting = (props) => {

	const { delegate } = props;

	const [ votingPhase, setVotingPhase ] = useState("proposal");
	const [ votingPeriod, setVotingPeriod ] = useState(0);
	const [ remainingPhaseBlocks, setRemainingBlocks ] = useState(0);
	const [ explorationProposals, setExplorationProposals ] = useState([]);
	const [ currentProposal, setCurrentProposal ] = useState("");
	const [ hasVoted, setHasVoted ] = useState(false);
	const [ isCastingVote, setIsCastingVote ] = useState(false);
	const [ alert, setAlert ] = useState({});
	const [ isLoading, setIsLoading ] = useState(true);
	const addToast = useContext(ToasterContext);

	useEffect(() => {

		// Determine current period
		const currentPeriodApi = "https://mainnet-tezos.giganode.io/chains/main/blocks/1504327/votes/current_period"
		apiRequest(currentPeriodApi)
		.then((data) => {

			setVotingPeriod(data.voting_period.index);
			setVotingPhase(data.voting_period.kind || "Unknown");
			setRemainingBlocks(data.remaining);

			// proposal -> exploration -> cooldown -> promotion -> adoption

			if (votingPhase === "proposal") {
				// Fetch current list of proposals and display
				const activeProposalsApi = "https://mainnet-tezos.giganode.io/chains/main/blocks/1504327/votes/proposals"
				apiRequest(activeProposalsApi)
				.then((data) => {
					setExplorationProposals(data);
				})
				.catch((errMsg) => { throw errMsg; })
			}

			if (votingPhase === "exploration" || votingPhase === "cooldown" || votingPhase === "promotion") {

				// Fetch current proposal
				const currentProposalApi = "https://mainnet-tezos.giganode.io/chains/main/blocks/1504327/votes/current_proposal"
				apiRequest(currentProposalApi)
				.then((data) => {
					setCurrentProposal(data)
				})
				.catch((errMsg) => { throw errMsg; })

				// If exploration, get list of votes to check if already voted
				if (votingPhase === "exploration") {
					const ballotListApi = "https://mainnet-tezos.giganode.io/chains/main/blocks/1509400/votes/ballot_list"
					apiRequest(ballotListApi)
					.then((data) => {
						const ballots = data;
						ballots.forEach((e) => {
							if (e.pkh === delegate) {
								setHasVoted(true);
							}
						})
					})
					.catch((errMsg) => { throw errMsg; })
				}
			}
		})
		.catch((errMsg) => {
			console.log(errMsg);
			addToast({
				title: "Loading Voting Period Error",
				msg: errMsg,
				type: "danger",
			});
		})
		.finally(() => {
			setIsLoading(false);
		});

		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const castUpVote = (proposal) => {

		setIsCastingVote(true);

		// TODO: Check if already voted?
		
		const upVoteApiUrl = window.BASE_URL + "/api/voting/upvote"
		const requestOptions = {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				"p": proposal,
				"i": votingPeriod,
			})
		};
		
		apiRequest(upVoteApiUrl, requestOptions)
		.then((data) => {
			setAlert({
				type: "success",
				msg: "Successfully cast vote for proposal: "+proposal+"!"
			});
		})
		.catch((errMsg) => {
			console.log(errMsg);
			addToast({
				title: "Loading Voting Period Error",
				msg: errMsg,
				type: "danger",
			});
		})
		.finally(() => {
			setIsCastingVote(false);
		});
	}
	
	const castYayNayPassVote = (proposal, vote) => {
		console.log(proposal);
		console.log(vote);
	}


	if (isLoading) {
		return (
			<Row>
				<Col className="text-center padded-top-30">
					<Loader type="Circles" color="#EFC700" height={50} width={50} /><br/>Loading Voting Info...
				</Col>
			</Row>
		)
	}

	if (votingPhase === "proposal") {
		return (
			<Row>
				<Col>
					<Card>
						<Card.Header as="h5">Voting Governance</Card.Header>
						<Card.Body>
							<Card.Title>Proposal Period</Card.Title>
							<Card.Text>Tezos is currently in the <em>proposal</em> period where bakers can submit new protocols for voting, and cast votes on existing proposals. If any have been submitted so far, you can see them below, and can cast your vote using the button.</Card.Text>
							<Card.Text>This period will last for {remainingPhaseBlocks} more blocks. After this time, the proposal with the most votes will move into the <em>exploration</em> period where you will be able to cast another vote. If no proposals have been submitted by then, the proposal phase starts over.</Card.Text>

							{ isCastingVote ? 
							<>
							<Row>
								<Col className="text-center">
									<Loader type="Circles" color="#EFC700" height={25} width={25} />
									<br/>
									Casting your vote. If using a ledger device, please look at the device and confirm the vote.
								</Col>
							</Row>
							</>
							: 
							<>
							<Card.Header>Current Proposals</Card.Header>
							<ListGroup variant="flush">
							  { explorationProposals.map((o, i) => {
								  return <ListGroup.Item key={i}><Row>
									<Col md={1}><Button onClick={() => castUpVote(o[0])} variant="info" size="sm" type="button">Vote</Button></Col>
									<Col md={6}><Card.Link href={"https://agora.tezos.com/proposal/"+o[0]} target="_blank" rel="noreferrer">{o[0]}</Card.Link></Col>
								  </Row></ListGroup.Item>
							  })}
							</ListGroup>
							</>
							}
						</Card.Body>
					</Card>
				</Col>
			</Row>
		)
	}

	if (votingPhase === "exploration") {
		return (
			<Row>
				<Col>
					<Card>
						<Card.Header as="h5">Voting Governance</Card.Header>
						<Card.Body>
							<Card.Title>Exploration Period</Card.Title>
							<Card.Text>Tezos is currently in the <em>exploration</em> period where bakers can vote a new protocol.</Card.Text>
							<Card.Text>This period will last for {remainingPhaseBlocks} more blocks.</Card.Text>
							<Card.Text>Below, you will see the currently proposed protocol. You can click on the proposal to view the publicly posted information. Cast your vote by clicking on the appropriate button next to the proposal.</Card.Text>
							<Card.Header>Current Proposal</Card.Header>						
							<Row>
								  <Col md={1}><Button disabled={hasVoted} onClick={() => castYayNayPassVote('yay')} variant="info" size="sm" type="button">Yay</Button></Col>
								  <Col md={1}><Button disabled={hasVoted} onClick={() => castYayNayPassVote('nay')} variant="info" size="sm" type="button">Nay</Button></Col>
								  <Col md={1}><Button disabled={hasVoted} onClick={() => castYayNayPassVote('pass')} variant="info" size="sm" type="button">Pass</Button></Col>
								  <Col md={6}><Card.Link href={"https://agora.tezos.com/proposal/"+currentProposal} target="_blank" rel="noreferrer">{currentProposal}</Card.Link></Col>
							</Row>
							{ hasVoted && <Row><Col><Alert variant="secondary">You have already voted!</Alert></Col></Row> }
						</Card.Body>
					</Card>
				</Col>
			</Row>
		)
	}

	if (votingPhase === "cooldown") {
		return (
			<Row>
				<Col>
					<Card>
						<Card.Header as="h5">Voting Governance</Card.Header>
						<Card.Body>
							<Card.Text>There is currently no active voting period. Tezos is currently in the <em>cooldown</em> period where the initially chosen protocol, {currentProposal}, is being tested.</Card.Text>
							<Card.Text>This period will last for {remainingPhaseBlocks} more blocks. After this, voting will open once again for the <em>promotion</em> period.</Card.Text>
						</Card.Body>
					</Card>
				</Col>
			</Row>
		)
	}
	
	if (votingPhase === "adoption") {
		return (
			<Row>
				<Col>
					<Card>
						<Card.Header as="h5">Voting Governance</Card.Header>
						<Card.Body>
							<Card.Text>There is currently no active voting period. Tezos is currently in the <em>proposal</em> period where bakers can submit new protocols for voting.</Card.Text>
							<Card.Text>This period will last for {remainingPhaseBlocks} more blocks. If no proposals have been submitted by then, the proposal phase starts over.</Card.Text>
						</Card.Body>
					</Card>
				</Col>
			</Row>
		)
	}


	return (
		<Row><Col>Uh oh... something went wrong. You should refresh your browser and start over.</Col></Row>
	)
};

export default Voting;
