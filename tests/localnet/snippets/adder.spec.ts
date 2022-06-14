import { FiveMinutesInMilliseconds, INetworkConfig, INetworkProvider, ITestSession, ITestUser, TestSession } from "@elrondnetwork/erdjs-snippets";
import { Address, DefaultGasConfiguration, GasEstimator, IAddress, TokenPayment, Transaction, TransactionFactory, TransactionPayload, TransactionWatcher } from "@elrondnetwork/erdjs";
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

        await session.syncUsers([alice, bob]);

        let addressInShard0 = await deploy(bob);
        await session.saveAddress({ name: "adderInShard0", address: addressInShard0 });

        let addressInShard1 = await deploy(alice);
        await session.saveAddress({ name: "adderInShard1", address: addressInShard1 });
    });

    async function deploy(deployer: ITestUser): Promise<IAddress> {
        let interactor = await createInteractor(session);
        let { address, returnCode } = await interactor.deploy(deployer, 42);
        assert.isTrue(returnCode.isSuccess());
        return address;
    }

    it("add", async function () {
        this.timeout(FiveMinutesInMilliseconds);
        // If the step fails, retry it (using a Mocha utility function).
        this.retries(5);

        await session.syncUsers([bob]);

        let contractAddress = await session.loadAddress("adderInShard0");
        let interactor = await createInteractor(session, contractAddress);
        let returnCode = await interactor.add(bob, 3);
        assert.isTrue(returnCode.isSuccess());

        contractAddress = await session.loadAddress("adderInShard1");
        interactor = await createInteractor(session, contractAddress);
        returnCode = await interactor.add(bob, 3);
        assert.isTrue(returnCode.isSuccess());
    });

    it("send value to non-payable (intra-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        let contractAddress = await session.loadAddress("adderInShard0");

        const { transaction, transactionHash } = await sendToContract(bob, contractAddress, 1, "hello@aa.bb.cc", 10000000)
        console.log("Transaction:", transactionHash);

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("send value to non-payable (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        let contractAddress = await session.loadAddress("adderInShard1");

        const { transaction, transactionHash } = await sendToContract(bob, contractAddress, 1, "hello@aa.bb.cc", 10000000);
        console.log("Transaction:", transactionHash);

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("call bad function (intra-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        let contractAddress = await session.loadAddress("adderInShard0");

        const { transaction, transactionHash } = await sendToContract(bob, contractAddress, 1, "hello", 10000000)
        console.log("Transaction:", transactionHash);

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("call bad function (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        let contractAddress = await session.loadAddress("adderInShard1");

        const { transaction, transactionHash } = await sendToContract(bob, contractAddress, 1, "hello", 10000000)
        console.log("Transaction:", transactionHash);

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("call, too much gas (intra-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        let contractAddress = await session.loadAddress("adderInShard0");

        const { transaction, transactionHash } = await sendToContract(bob, contractAddress, 1, "add@01", 100000000)
        console.log("Transaction:", transactionHash);

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("call, too much gas (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        let contractAddress = await session.loadAddress("adderInShard1");

        const { transaction, transactionHash } = await sendToContract(bob, contractAddress, 1, "add@01", 100000000)
        console.log("Transaction:", transactionHash);

        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    async function sendToContract(user: ITestUser, contractAddress: IAddress, value: BigNumber.Value, data: string, gasLimit: number): Promise<{ transaction: Transaction, transactionHash: string }> {
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
