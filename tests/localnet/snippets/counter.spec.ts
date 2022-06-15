import { TokenPayment } from "@elrondnetwork/erdjs";
import { createAirdropService, createESDTInteractor, FiveMinutesInMilliseconds, INetworkProvider, ITestSession, ITestUser, TestSession } from "@elrondnetwork/erdjs-snippets";
import { assert } from "chai";
import { createInteractor } from "./counterInteractor";

describe("counter snippet", async function () {
    this.bail(true);

    let session: ITestSession;
    let provider: INetworkProvider;
    
    // shard(alice) = 1
    // shard(bob) = 0
    // shard(carol) = 2
    // shard(dan) = 1
    // shard(mike) = 0
    let owner: ITestUser;
    let alice: ITestUser;
    let bob: ITestUser;
    let carol: ITestUser;
    let dan: ITestUser;
    let mike: ITestUser;


    this.beforeAll(async function () {
        session = await TestSession.load("localnet", __dirname);
        provider = session.networkProvider;
        await session.syncNetworkConfig();

        owner = session.users.getUser("bob");
        alice = session.users.getUser("alice");
        bob = session.users.getUser("bob");
        carol = session.users.getUser("carol");
        dan = session.users.getUser("dan");
        mike = session.users.getUser("mike");
    });

    this.beforeEach(async function () {
        session.correlation.step = this.currentTest?.fullTitle() || "";
    });

    it("issue counter token", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        let interactor = await createESDTInteractor(session);
        await session.syncUsers([owner]);
        let token = await interactor.issueFungibleToken(owner, { name: "COUNTER", ticker: "COUNTER", decimals: 0, supply: "100000000" });
        await session.saveToken({ name: "counterToken", token: token });
    });

    it("airdrop counter token", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        let token = await session.loadToken("counterToken");
        let payment = TokenPayment.fungibleFromAmount(token.identifier, "100", token.decimals);
        await session.syncUsers([owner]);
        await createAirdropService(session).sendToEachUser(owner, [alice, bob, mike], [payment]);
    });

    it("setup", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([owner]);

        let interactor = await createInteractor(session);
        let { address, returnCode } = await interactor.deploy(owner, 42);

        assert.isTrue(returnCode.isSuccess());

        await session.saveAddress({ name: "counter", address: address });
    });

    it("increment with single ESDT transfer", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([owner, alice, mike]);

        let contractAddress = await session.loadAddress("counter");
        let token = await session.loadToken("counterToken");
        let interactor = await createInteractor(session, contractAddress);

        let payment = TokenPayment.fungibleFromAmount(token.identifier, "10", token.decimals);

        interactor.incrementWithSingleESDTTransfer(owner, 1, payment);
        interactor.incrementWithSingleESDTTransfer(alice, 1, payment);
        interactor.incrementWithSingleESDTTransfer(mike, 1, payment);
    });

    it("increment with multi transfer", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice, bob, mike]);

        let contractAddress = await session.loadAddress("counter");
        let token = await session.loadToken("counterToken");
        let interactor = await createInteractor(session, contractAddress);

        let payment = TokenPayment.fungibleFromAmount(token.identifier, "10", token.decimals);

        // Intra-shard
        interactor.incrementWithMultiTransfer(bob, 1, payment);

        // Cross-shard
        interactor.incrementWithSingleESDTTransfer(mike, 1, payment);
    });

    it("destroy session", async function () {
        await session.destroy();
    });
});
