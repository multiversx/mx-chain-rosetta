import { Address, DefaultGasConfiguration, GasEstimator, ReturnCode, TokenPayment, Transaction, TransactionFactory, TransactionWatcher } from "@elrondnetwork/erdjs";
import { FiveMinutesInMilliseconds, IAddress, INetworkConfig, INetworkProvider, ITestSession, ITestUser, ITransaction, TestSession } from "@elrondnetwork/erdjs-snippets";
import { assert } from "chai";
import { createAdderInteractor } from "./adderInteractor";
import { CustomNetworkProvider } from "./customNetworkProvider";

describe("scheduled", async function () {
    this.bail(true);

    let session: ITestSession;
    let networkProvider: INetworkProvider;
    let customNetworkProvider: CustomNetworkProvider;
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

    let manyZero: ITestUser[];
    let manyOne: ITestUser[];
    let manyTwo: ITestUser[];
    let manyUsers: ITestUser[];

    this.beforeAll(async function () {
        session = await TestSession.load("localnet", __dirname);
        await session.syncNetworkConfig();

        networkProvider = session.networkProvider;
        customNetworkProvider = new CustomNetworkProvider();
        networkConfig = session.getNetworkConfig();
        transactionFactory = new TransactionFactory(new GasEstimator(DefaultGasConfiguration));
        transactionWatcher = new TransactionWatcher(networkProvider);

        alice = session.users.getUser("alice");
        bob = session.users.getUser("bob");
        carol = session.users.getUser("carol");
        dan = session.users.getUser("dan");
        mike = session.users.getUser("mike");
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
        await session.syncUsers([alice]);
        
        let transactions: Transaction[] = [];

        for (const receiver of manyUsers) {
            const transaction = transactionFactory.createEGLDTransfer({
                nonce: alice.account.getNonceThenIncrement(),
                sender: alice.address,
                receiver: receiver.address,
                value: payment.toString(),
                chainID: networkConfig.ChainID
            });

            await alice.signer.sign(transaction);
            transactions.push(transaction);
        }

        await customNetworkProvider.sendTransactions(transactions);
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

    it("send many contract transactions (some invalid) - intrashard", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers(manyZero);
        await session.syncUsers(manyOne);
        await session.syncUsers(manyTwo);

        const contractAddress = await session.loadAddress("adderInShard0");
        const interactor = await createAdderInteractor(session, contractAddress);

        const transactions: ITransaction[] = [];

        for (const user of manyUsers) {
            transactions.push(await interactor.buildTransactionAdd(user, 1));
        }

        for (const sender of manyZero.slice(0, 5)) {
            const transaction = transactionFactory.createESDTTransfer({
                nonce: sender.account.getNonceThenIncrement(),
                sender: sender.address,
                receiver: bob.address,
                payment: TokenPayment.fungibleFromAmount("ABBA-abba", 10000000, 5),
                chainID: networkConfig.ChainID
            });

            await sender.signer.sign(transaction);
            console.log("Transaction will be in miniblock of type Invalid:", transaction.getHash().hex());
            transactions.push(transaction);
        }
        
        await customNetworkProvider.sendTransactions(transactions);
    });

    it("generate report", async function () {
        await session.generateReport();
    });

    it("destroy session", async function () {
        await session.destroy();
    });
});
