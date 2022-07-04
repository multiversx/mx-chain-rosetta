import { TokenPayment } from "@elrondnetwork/erdjs";
import { createAirdropService, FiveMinutesInMilliseconds, INetworkProvider, ITestSession, ITestUser, TestSession } from "@elrondnetwork/erdjs-snippets";

describe("transfers snippet", async function () {
    this.bail(true);

    let session: ITestSession;
    let provider: INetworkProvider;
    let alice: ITestUser;
    let bob: ITestUser;

    this.beforeAll(async function () {
        session = await TestSession.load("localnet", __dirname);
        provider = session.networkProvider;
        alice = session.users.getUser("alice");
        bob = session.users.getUser("bob");
        await session.syncNetworkConfig();
    });

    it("transfer NFT", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        let payment = TokenPayment.nonFungible("TEST-38f249", 1);
        await session.syncUsers([bob]);
        await createAirdropService(session).sendToEachUser(bob, [alice], [payment]);
    });
});
