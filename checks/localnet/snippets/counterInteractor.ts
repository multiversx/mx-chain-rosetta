import path from "path";
import { BigUIntValue, CodeMetadata, IAddress, Interaction, ResultsParser, ReturnCode, SmartContract, SmartContractAbi, TokenPayment, TransactionWatcher, U64Value } from "@elrondnetwork/erdjs";
import { INetworkConfig, INetworkProvider, ITestSession, ITestUser, loadAbiRegistry, loadCode } from "@elrondnetwork/erdjs-snippets";

const PathToWasm = path.resolve(__dirname, "counter.wasm");
const PathToAbi = path.resolve(__dirname, "counter.abi.json");

export async function createInteractor(session: ITestSession, contractAddress?: IAddress): Promise<CounterInteractor> {
    let registry = await loadAbiRegistry(PathToAbi);
    let abi = new SmartContractAbi(registry);
    let contract = new SmartContract({ address: contractAddress, abi: abi });
    let networkProvider = session.networkProvider;
    let networkConfig = session.getNetworkConfig();
    let interactor = new CounterInteractor(contract, networkProvider, networkConfig);
    return interactor;
}

export class CounterInteractor {
    private readonly contract: SmartContract;
    private readonly networkProvider: INetworkProvider;
    private readonly networkConfig: INetworkConfig;
    private readonly transactionWatcher: TransactionWatcher;
    private readonly resultsParser: ResultsParser;

    constructor(contract: SmartContract, networkProvider: INetworkProvider, networkConfig: INetworkConfig) {
        this.contract = contract;
        this.networkProvider = networkProvider;
        this.networkConfig = networkConfig;
        this.transactionWatcher = new TransactionWatcher(networkProvider);
        this.resultsParser = new ResultsParser();
    }

    async deploy(deployer: ITestUser, initialValue: number): Promise<{ address: IAddress, returnCode: ReturnCode }> {
        // Load the bytecode.
        let code = await loadCode(PathToWasm);

        // Prepare the deploy transaction.
        let transaction = this.contract.deploy({
            code: code,
            codeMetadata: new CodeMetadata(),
            initArguments: [new BigUIntValue(initialValue)],
            gasLimit: 5000000,
            chainID: this.networkConfig.ChainID
        });

        // Set the transaction nonce. The account nonce must be synchronized beforehand.
        // Also, locally increment the nonce of the deployer (optional).
        transaction.setNonce(deployer.account.getNonceThenIncrement());

        // Let's sign the transaction. For dApps, use a wallet provider instead.
        await deployer.signer.sign(transaction);

        // The contract address is deterministically computable:
        let address = SmartContract.computeAddress(transaction.getSender(), transaction.getNonce());

        // Let's broadcast the transaction and await its completion:
        await this.networkProvider.sendTransaction(transaction);
        let transactionOnNetwork = await this.transactionWatcher.awaitCompleted(transaction);

        // In the end, parse the results:
        let { returnCode } = this.resultsParser.parseUntypedOutcome(transactionOnNetwork);

        console.log(`CounterInteractor.deploy(): contract = ${address}`);
        return { address, returnCode };
    }

    async incrementWithSingleESDTTransfer(caller: ITestUser, value: number, payment: TokenPayment): Promise<ReturnCode> {
        // Prepare the interaction
        let interaction = <Interaction>this.contract.methods
            .increment([new U64Value(value)])
            .withSingleESDTTransfer(payment)
            .withGasLimit(3000000)
            .withNonce(caller.account.getNonceThenIncrement())
            .withChainID(this.networkConfig.ChainID);

        // Let's build the transaction object.
        let transaction = interaction.buildTransaction();

        // Let's sign the transaction. For dApps, use a wallet provider instead.
        await caller.signer.sign(transaction);

        // Let's broadcast the transaction and await its completion:
        await this.networkProvider.sendTransaction(transaction);
        let transactionOnNetwork = await this.transactionWatcher.awaitCompleted(transaction);

        // In the end, parse the results:
        let { returnCode } = this.resultsParser.parseOutcome(transactionOnNetwork, interaction.getEndpoint());
        return returnCode;
    }

    async incrementWithMultiTransfer(caller: ITestUser, value: number, payment: TokenPayment): Promise<ReturnCode> {
        // Prepare the interaction
        let interaction = <Interaction>this.contract.methods
            .increment([new U64Value(value)])
            .withMultiESDTNFTTransfer([payment], caller.address)
            .withGasLimit(3000000)
            .withNonce(caller.account.getNonceThenIncrement())
            .withChainID(this.networkConfig.ChainID);

        // Let's build the transaction object.
        let transaction = interaction.buildTransaction();

        // Let's sign the transaction. For dApps, use a wallet provider instead.
        await caller.signer.sign(transaction);

        // Let's broadcast the transaction and await its completion:
        await this.networkProvider.sendTransaction(transaction);
        let transactionOnNetwork = await this.transactionWatcher.awaitCompleted(transaction);

        // In the end, parse the results:
        let { returnCode } = this.resultsParser.parseOutcome(transactionOnNetwork, interaction.getEndpoint());
        return returnCode;
    }
}
