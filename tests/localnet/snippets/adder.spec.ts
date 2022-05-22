import { FiveMinutesInMilliseconds, INetworkProvider, ITestSession, ITestUser, TestSession } from "@elrondnetwork/erdjs-snippets";
import { assert } from "chai";
import { createInteractor } from "./adderInteractor";

describe("adder snippet", async function () {
    this.bail(true);

    let session: ITestSession;
    let provider: INetworkProvider;
    let alice: ITestUser;

    this.beforeAll(async function () {
        session = await TestSession.load("localnet", __dirname);
        provider = session.networkProvider;
        alice = session.users.getUser("alice");
        await session.syncNetworkConfig();
    });

    this.beforeEach(async function () {
        session.correlation.step = this.currentTest?.fullTitle() || "";
    });

    it("setup", async function () {
        this.timeout(FiveMinutesInMilliseconds);

        await session.syncUsers([alice]);

        let interactor = await createInteractor(session);
        let { address, returnCode } = await interactor.deploy(alice, 42);

        assert.isTrue(returnCode.isSuccess());

        await session.saveAddress({ name: "adder", address: address });
    });

    it("add", async function () {
        this.timeout(FiveMinutesInMilliseconds);
        // If the step fails, retry it (using a Mocha utility function).
        this.retries(5);

        await session.syncUsers([alice]);

        const contractAddress = await session.loadAddress("adder");
        const interactor = await createInteractor(session, contractAddress);

        const sumBefore = await interactor.getSum();
        const snapshotBefore = await session.audit.onSnapshot({ state: { sum: sumBefore } });

        const returnCode = await interactor.add(alice, 3);
        await session.audit.onContractOutcome({ returnCode });

        const sumAfter = await interactor.getSum();
        await session.audit.onSnapshot({ state: { sum: sumBefore }, comparableTo: snapshotBefore });

        assert.isTrue(returnCode.isSuccess());
        assert.equal(sumAfter, sumBefore + 3);
    });

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
