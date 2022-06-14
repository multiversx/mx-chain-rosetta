import { FiveMinutesInMilliseconds, INetworkConfig, INetworkProvider, ITestSession, ITestUser, TestSession } from "@elrondnetwork/erdjs-snippets";
import { DefaultGasConfiguration, GasEstimator, TokenPayment, Transaction, TransactionFactory, TransactionPayload, TransactionWatcher } from "@elrondnetwork/erdjs";
import { assert } from "chai";
import { createInteractor } from "./adderInteractor";
import BigNumber from "bignumber.js";

describe("adder snippet", async function () {
    this.bail(true);

    let session: ITestSession;
    let provider: INetworkProvider;
    let networkConfig: INetworkConfig;
    let transactionFactory: TransactionFactory;
    let transactionWatcher: TransactionWatcher;

    // shard(alice) = 1
    // shard(bob) = 0
    let alice: ITestUser;
    let bob: ITestUser;

    this.beforeAll(async function () {
        session = await TestSession.load("localnet", __dirname);
        await session.syncNetworkConfig();

        provider = session.networkProvider;
        networkConfig = session.getNetworkConfig();
        transactionFactory = new TransactionFactory(new GasEstimator(DefaultGasConfiguration));
        transactionWatcher = new TransactionWatcher(provider);

        alice = session.users.getUser("alice");
        bob = session.users.getUser("bob");
    });

    this.beforeEach(async function () {
        session.correlation.step = this.currentTest?.fullTitle() || "";
    });

    it("setup", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        let interactor = await createInteractor(session);
        let { address, returnCode } = await interactor.deploy(bob, 42);

        assert.isTrue(returnCode.isSuccess());

        await session.saveAddress({ name: "adder", address: address });
    });

    it("add", async function () {
        this.timeout(FiveMinutesInMilliseconds);
        // If the step fails, retry it (using a Mocha utility function).
        this.retries(5);

        await session.syncUsers([bob]);

        const contractAddress = await session.loadAddress("adder");
        const interactor = await createInteractor(session, contractAddress);

        const sumBefore = await interactor.getSum();
        const snapshotBefore = await session.audit.onSnapshot({ state: { sum: sumBefore } });

        const returnCode = await interactor.add(bob, 3);
        await session.audit.onContractOutcome({ returnCode });

        const sumAfter = await interactor.getSum();
        await session.audit.onSnapshot({ state: { sum: sumBefore }, comparableTo: snapshotBefore });

        assert.isTrue(returnCode.isSuccess());
        assert.equal(sumAfter, sumBefore + 3);
    });

    it("send value to non-payable (intra-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        const {transaction, transactionHash } = await sendToContract(bob, 1, "hello@aa.bb.cc", 10000000)
        session.audit.onTransactionSent({ action: "bob to non-payable contract", transactionHash });

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("send value to non-payable (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice]);

        const {transaction, transactionHash } = await sendToContract(alice, 1, "hello@aa.bb.cc", 10000000)
        session.audit.onTransactionSent({ action: "alice to non-payable contract", transactionHash });

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("call bad function (intra-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        const {transaction, transactionHash } = await sendToContract(bob, 1, "hello", 10000000)
        session.audit.onTransactionSent({ action: "bob, bad call", transactionHash });

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("call bad function (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice]);

        const {transaction, transactionHash } = await sendToContract(alice, 1, "hello", 10000000)
        session.audit.onTransactionSent({ action: "alice, bad call", transactionHash });

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("call, too much gas (intra-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        const {transaction, transactionHash } = await sendToContract(bob, 1, "add@01", 100000000)
        session.audit.onTransactionSent({ action: "bob, too much gas", transactionHash });

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("call, too much gas (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice]);

        const {transaction, transactionHash } = await sendToContract(alice, 1, "add@01", 100000000)
        session.audit.onTransactionSent({ action: "alice, too much gas", transactionHash });

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    async function sendToContract(user: ITestUser, value: BigNumber.Value, data: string, gasLimit: number): Promise<{ transaction: Transaction, transactionHash: string }> {
        const contractAddress = await session.loadAddress("adder");
        const payment = TokenPayment.egldFromAmount(value);

        const transaction = transactionFactory.createEGLDTransfer({
            nonce: user.account.getNonceThenIncrement(),
            sender: user.address,
            receiver: contractAddress,
            value: payment.toString(),
            chainID: networkConfig.ChainID,
            gasLimit: gasLimit,
            data: new TransactionPayload(data)
        });

        await user.signer.sign(transaction);
        const transactionHash = await provider.sendTransaction(transaction);
        return { transaction, transactionHash };
    }

    it("getSum", async function () {
        let contractAddress = await session.loadAddress("adder");
        let interactor = await createInteractor(session, contractAddress);
        let result = await interactor.getSum();
        assert.isTrue(result > 0);
    });

    it("generate report", async function () {
        await session.generateReport();
    });

    it("destroy session", async function () {
        await session.destroy();
    });
});
