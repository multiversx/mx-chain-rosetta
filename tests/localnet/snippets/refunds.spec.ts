import { DefaultGasConfiguration, GasEstimator, TokenPayment, TransactionFactory, TransactionWatcher } from "@elrondnetwork/erdjs";
import { FiveMinutesInMilliseconds, INetworkConfig, INetworkProvider, ITestSession, ITestUser, TestSession } from "@elrondnetwork/erdjs-snippets";

describe("refunds", async function () {
    this.bail(true);

    let session: ITestSession;
    let networkProvider: INetworkProvider;
    let networkConfig: INetworkConfig;
    let transactionFactory: TransactionFactory;
    let transactionWatcher: TransactionWatcher;

    // shard(alice) = 1
    // shard(bob) = 0
    // shard(carol) = 2
    // shard(dan) = 1
    let alice: ITestUser;
    let bob: ITestUser;
    let carol: ITestUser;
    let dan: ITestUser;

    this.beforeAll(async function () {
        session = await TestSession.load("localnet", __dirname);
        await session.syncNetworkConfig();

        networkProvider = session.networkProvider;
        networkConfig = session.getNetworkConfig();
        transactionFactory = new TransactionFactory(new GasEstimator(DefaultGasConfiguration));
        transactionWatcher = new TransactionWatcher(networkProvider);

        alice = session.users.getUser("alice");
        bob = session.users.getUser("bob");
        carol = session.users.getUser("carol");
        dan = session.users.getUser("dan");
    });

    this.beforeEach(async function () {
        session.correlation.step = this.currentTest?.fullTitle() || "";
    });

    it("simple move balance with refund (intra-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice]);

        const payment = TokenPayment.egldFromAmount(1);

        // Cross-shard
        const transaction = transactionFactory.createEGLDTransfer({
            nonce: alice.account.nonce,
            value: payment,
            receiver: dan.address,
            gasLimit: 100000,
            chainID: networkConfig.ChainID
        });

        await alice.signer.sign(transaction)

        const transactionHash = await networkProvider.sendTransaction(transaction);
        session.audit.onTransactionSent({ action: "alice to dan", transactionHash });
        
        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("simple move balance with refund (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice]);

        const payment = TokenPayment.egldFromAmount(1);

        // Cross-shard
        const transaction = transactionFactory.createEGLDTransfer({
            nonce: alice.account.nonce,
            value: payment,
            receiver: bob.address,
            gasLimit: 100000,
            chainID: networkConfig.ChainID
        });

        await alice.signer.sign(transaction)

        const transactionHash = await networkProvider.sendTransaction(transaction);
        session.audit.onTransactionSent({ action: "alice to bob", transactionHash });
        
        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("generate report", async function () {
        await session.generateReport();
    });

    it("destroy session", async function () {
        await session.destroy();
    });
});
