import { Address, DefaultGasConfiguration, GasEstimator, TokenPayment, TransactionFactory, TransactionWatcher } from "@elrondnetwork/erdjs";
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
    // shard(mike) = 0
    let alice: ITestUser;
    let bob: ITestUser;
    let carol: ITestUser;
    let dan: ITestUser;
    let mike: ITestUser;

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
        mike = session.users.getUser("mike");
    });

    this.beforeEach(async function () {
        session.correlation.step = this.currentTest?.fullTitle() || "";
    });

    it("simple move balance with refund (intra-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        const payment = TokenPayment.egldFromAmount(1);

        const transaction = transactionFactory.createEGLDTransfer({
            nonce: bob.account.nonce,
            value: payment,
            receiver: mike.address,
            gasLimit: 100000,
            chainID: networkConfig.ChainID
        });

        await bob.signer.sign(transaction)

        const transactionHash = await networkProvider.sendTransaction(transaction);
        session.audit.onTransactionSent({ action: "bob to mike", transactionHash });
        
        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("simple move balance with refund (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([bob]);

        const payment = TokenPayment.egldFromAmount(1);

        const transaction = transactionFactory.createEGLDTransfer({
            nonce: bob.account.nonce,
            value: payment,
            receiver: alice.address,
            gasLimit: 100000,
            chainID: networkConfig.ChainID
        });

        await bob.signer.sign(transaction)

        const transactionHash = await networkProvider.sendTransaction(transaction);
        session.audit.onTransactionSent({ action: "bob to alice", transactionHash });
        
        const transactionOnNetwork = await transactionWatcher.awaitCompleted(transaction);
        session.audit.onTransactionCompleted({ transactionHash, transaction: transactionOnNetwork });
    });

    it("invalid transactions (cross-shard)", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice, bob]);

        const payment = TokenPayment.egldFromAmount(5000000);

        const bobToAlice = transactionFactory.createEGLDTransfer({
            nonce: bob.account.getNonceThenIncrement(),
            value: payment,
            receiver: alice.address,
            gasLimit: 100000,
            chainID: networkConfig.ChainID
        });
        await bob.signer.sign(bobToAlice);

        let transactionHash = await networkProvider.sendTransaction(bobToAlice);
        console.log("Bob to Alice:", transactionHash)
        await transactionWatcher.awaitCompleted(bobToAlice);
    });

    it("invalid transactions (intra-shard)", async function () {
        // 99899999900000000000000 (initial)
        // 99899999849500000000000 (invalid miniblock)
        //      -> the actual fee is 50500000000000
        //      -> note that a receipt with data = "insufficient funds" is generated (its value field refers to the fee, thus is not relevant)

        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice, bob]);

        const payment = TokenPayment.egldFromAmount(5000000);

        const transactionBobToBob = transactionFactory.createEGLDTransfer({
            nonce: bob.account.getNonceThenIncrement(),
            value: payment,
            receiver: bob.address,
            gasLimit: 100000,
            chainID: networkConfig.ChainID
        });
        await bob.signer.sign(transactionBobToBob);

        let transactionHash = await networkProvider.sendTransaction(transactionBobToBob);
        console.log("Bob to Bob:", transactionHash)
        await transactionWatcher.awaitCompleted(transactionBobToBob);
    });

    it("sending value to non-payable (cross-shard)", async function () {
        // 99900000000000000000000 (initial)
        // 99898999950000000000000 (at first, at source: 1 EGLD + fee only for move balance)
        //      -> note that a receipt of 9500000000000 is generated here
        // 99899999950000000000000 (1 EGLD sent back)
        // Note that this works a bit different for epoch < penalizeTooMuchGasEnableEpoch
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice, bob, carol]);

        const payment = TokenPayment.egldFromAmount(1);

        // Cross-shard (0 to 1)
        const bobToDelegation = transactionFactory.createEGLDTransfer({
            nonce: bob.account.getNonceThenIncrement(),
            value: payment,
            receiver: new Address("erd1qqqqqqqqqqqqqpgqak8zt22wl2ph4tswtyc39namqx6ysa2sd8ss4xmlj3"),
            gasLimit: 1000000,
            chainID: networkConfig.ChainID
        });
        await bob.signer.sign(bobToDelegation);

        let transactionHash = await networkProvider.sendTransaction(bobToDelegation);
        console.log("Bob to delegation:", transactionHash)
        await transactionWatcher.awaitCompleted(bobToDelegation);
    });

    it("sending value to non-payable (intra-shard)", async function () {
        // 99899999950000000000000 (initial)
        // 99899999900000000000000 (after execution at source, invalid miniblock)
        //      -> note that a log is generated in this case, with event.data = "@hex('sending value to non payable contract')..."
        //      -> fee is only for move balance: 50000000000000
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice, bob, carol]);

        const payment = TokenPayment.egldFromAmount(1);

        // Intra-shard (0 to 0)
        const transactionBobToDNS = transactionFactory.createEGLDTransfer({
            nonce: bob.account.getNonceThenIncrement(),
            value: payment,
            receiver: new Address("erd1qqqqqqqqqqqqqpgqnhvsujzd95jz6fyv3ldmynlf97tscs9nqqqq49en6w"),
            gasLimit: 100000,
            chainID: networkConfig.ChainID
        });
        await bob.signer.sign(transactionBobToDNS);

        let transactionHash = await networkProvider.sendTransaction(transactionBobToDNS);
        console.log("Bob to DNS:", transactionHash)
        await transactionWatcher.awaitCompleted(transactionBobToDNS);
    });

    it("generate report", async function () {
        await session.generateReport();
    });

    it("destroy session", async function () {
        await session.destroy();
    });
});
