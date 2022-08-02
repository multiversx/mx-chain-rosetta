import { DefaultGasConfiguration, GasEstimator, TokenPayment, Transaction, TransactionFactory, TransactionWatcher } from "@elrondnetwork/erdjs";
import { ProxyNetworkProvider } from "@elrondnetwork/erdjs-network-providers/out";
import { createAirdropService, createESDTInteractor, FiveMinutesInMilliseconds, IAddress, INetworkConfig, ITestSession, ITestUser, ITransaction, TestSession } from "@elrondnetwork/erdjs-snippets";
import { assert } from "chai";
import { createAdderInteractor } from "./adderInteractor";

describe("scheduled", async function () {
    this.bail(true);

    let session: ITestSession;
    let networkProvider: ProxyNetworkProvider;
    let networkConfig: INetworkConfig;
    let transactionFactory: TransactionFactory;
    let transactionWatcher: TransactionWatcher;

    // shard(alice) = 1
    // shard(bob) = 0
    // shard(carol) = 2
    // shard(dan) = 1
    // shard(mike) = 0
    let whale: ITestUser;
    let alice: ITestUser;
    let bob: ITestUser;

    let manyZero: ITestUser[];
    let manyOne: ITestUser[];
    let manyTwo: ITestUser[];
    let manyUsers: ITestUser[];

    this.beforeAll(async function () {
        session = await TestSession.load("localnet", __dirname);
        await session.syncNetworkConfig();

        networkProvider = <ProxyNetworkProvider>session.networkProvider;
        networkConfig = session.getNetworkConfig();
        transactionFactory = new TransactionFactory(new GasEstimator(DefaultGasConfiguration));
        transactionWatcher = new TransactionWatcher(networkProvider);

        whale = session.users.getUser("whale");
        alice = session.users.getUser("alice");
        bob = session.users.getUser("bob");
        manyZero = session.users.getGroup("manyZero");
        manyOne = session.users.getGroup("manyOne");
        manyTwo = session.users.getGroup("manyTwo");
        manyUsers = [...manyZero, ...manyOne, ...manyTwo];
    });

    this.beforeEach(async function () {
        session.correlation.step = this.currentTest?.fullTitle() || "";
    });

    it("airdrop egld", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        const payment = TokenPayment.egldFromAmount(1);
        await session.syncUsers([whale]);
        
        let transactions: Transaction[] = [];

        for (const receiver of manyUsers) {
            const transaction = transactionFactory.createEGLDTransfer({
                nonce: whale.account.getNonceThenIncrement(),
                sender: whale.address,
                receiver: receiver.address,
                value: payment.toString(),
                chainID: networkConfig.ChainID
            });

            await whale.signer.sign(transaction);
            transactions.push(transaction);
        }

        await sendTransactions(transactions);

        // TODO: Add some waits. E.g. for the last transaction.
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
        let interactor = await createAdderInteractor(session);
        let { address, returnCode } = await interactor.deploy(deployer, 42);
        assert.isTrue(returnCode.isSuccess());
        return address;
    }

    it("invalid ESDT transfer and execute", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers(manyZero);
        const contractAddress = await session.loadAddress("adderInShard0");
        const interactor = await createAdderInteractor(session, contractAddress);

        const payment = TokenPayment.fungibleFromAmount("ABBA-abba", "10", 0);
        const transactions: ITransaction[] = [];

        for (const user of manyZero) {
            transactions.push(await interactor.buildTransactionTransferAdd(user, 1, 400000000, payment));
        }

        await sendTransactions(transactions);
    });

    it("issue token", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        const interactor = await createESDTInteractor(session);
        await session.syncUsers([whale]);

        const token = await interactor.issueFungibleToken(whale, { name: "FOO", ticker: "FOO", decimals: 0, supply: "100000000" });
        await session.saveToken({ name: "fooToken", token: token });
    });

    it("airdrop token", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        const token = await session.loadToken("fooToken");
        const payment = TokenPayment.fungibleFromAmount(token.identifier, "10", token.decimals);
        await session.syncUsers([whale]);

        await createAirdropService(session).sendToEachUser(whale, manyUsers, [payment]);
    });

    it("execute many", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers(manyUsers);

        const token = await session.loadToken("fooToken");
        const adderInShard0 = await session.loadAddress("adderInShard0");
        const adderInShard1 = await session.loadAddress("adderInShard1");
        const interactor0 = await createAdderInteractor(session, adderInShard0);
        const interactor1 = await createAdderInteractor(session, adderInShard1);

        const payment = TokenPayment.fungibleFromAmount(token.identifier, 1, token.decimals);
        const transactions: ITransaction[] = [];

        for (const user of manyZero) {
            transactions.push(await interactor0.buildTransactionTransferAdd(user, 1, 450000000, payment));
            transactions.push(await interactor1.buildTransactionTransferAdd(user, 1, 450000000, payment));
        }

        await sendTransactions(transactions);
    });

    async function sendTransactions(transactions: ITransaction[]): Promise<void> {
        const sendableTransactions = transactions.map(item => item.toSendable());
        const response = await networkProvider.doPostGeneric("transaction/send-multiple", sendableTransactions);
        console.log("Sent transactions:", response.numOfSentTxs);
        console.log(response);
    }
});
