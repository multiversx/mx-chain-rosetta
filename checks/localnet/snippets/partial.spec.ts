import { DefaultGasConfiguration, GasEstimator, TransactionFactory, TransactionWatcher } from "@elrondnetwork/erdjs";
import { ProxyNetworkProvider } from "@elrondnetwork/erdjs-network-providers/out";
import { FiveMinutesInMilliseconds, IAddress, INetworkConfig, ITestSession, ITestUser, ITransaction, TestSession } from "@elrondnetwork/erdjs-snippets";
import { assert } from "chai";
import { createAdderInteractor } from "./adderInteractor";

// For testing this scenario, a localnet should be used, with altered code (waits / time.Sleep() in processing).
describe("partial", async function () {
    this.bail(true);

    let session: ITestSession;
    let networkProvider: ProxyNetworkProvider;
    let networkConfig: INetworkConfig;
    let transactionFactory: TransactionFactory;
    let transactionWatcher: TransactionWatcher;

    // shard(alice) = 1
    // shard(bob) = 0
    let whale: ITestUser;
    let alice: ITestUser;
    let bob: ITestUser;

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
        let interactor = await createAdderInteractor(session);
        let { address, returnCode } = await interactor.deploy(deployer, 42);
        assert.isTrue(returnCode.isSuccess());
        return address;
    }

    it("execute many", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice, bob]);

        const adderInShard0 = await session.loadAddress("adderInShard0");
        const interactor0 = await createAdderInteractor(session, adderInShard0);

        const transactions: ITransaction[] = [];

        for (let i = 0; i < 10; i++) {
            transactions.push(await interactor0.buildTransactionAdd(alice, 1, 30000000));
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
